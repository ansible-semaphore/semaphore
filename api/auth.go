package api

import (
	"net/http"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/ansible-semaphore/semaphore/db"
	"github.com/ansible-semaphore/semaphore/util"
	"github.com/gorilla/context"
)

type authType int

const (
	authBearer authType = iota
	authCookie
	authExternal
	authUnknown
)

var authCookieName = "semaphore"

func determineAuthType(r *http.Request) authType {
	if len(r.Header.Get("authorization")) > 0 && strings.Contains(r.Header.Get("authorization"), "bearer") {
		return authBearer
	}

	if _, err := r.Cookie(authCookieName); err != nil {
		return authCookie
	}

	if util.Config.ExternalAuth && len(r.Header.Get(util.Config.ExternalAuthHeader.Username)) > 0 {
		return authExternal
	}

	return authUnknown
}

func authenticationHandler(w http.ResponseWriter, r *http.Request) bool {
	var userID int

	authType := determineAuthType(r)

	var user db.User

	switch authType {
	case authBearer:
		token, err := helpers.Store(r).GetAPIToken(
			strings.Replace(
				strings.ToLower(r.Header.Get("authorization")), "bearer ", "", 1))

		if err != nil {
			if err != db.ErrNotFound {
				log.Error(err)
			}

			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		user, err = fetchUser(helpers.Store(r), token.UserID)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		break
	case authCookie:
		// fetch session from cookie
		cookie, err := r.Cookie(authCookieName)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		value := make(map[string]interface{})
		if err = util.Cookie.Decode(authCookieName, cookie.Value, &value); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		user, ok := value["user"]
		sessionVal, okSession := value["session"]
		if !ok || !okSession {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		userID = user.(int)
		sessionID := sessionVal.(int)

		// fetch session
		session, err := helpers.Store(r).GetSession(userID, sessionID)

		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		if time.Since(session.LastActive).Hours() > 7*24 {
			// more than week old unused session
			// destroy.
			if err := helpers.Store(r).ExpireSession(userID, sessionID); err != nil {
				// it is internal error, it doesn't concern the user
				log.Error(err)
			}

			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		if err := helpers.Store(r).TouchSession(userID, sessionID); err != nil {
			log.Error(err)
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		user, err = fetchUser(helpers.Store(r), userID)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}

		break
	case authExternal:
		username := strings.ToLower(r.Header.Get(util.Config.ExternalAuthHeader.Username))

		// find user account
		user, err := helpers.Store(r).GetUserByLoginOrEmail(username, username)
		if err != nil {
			if err != db.ErrNotFound {
				log.Error(err)
				w.WriteHeader(http.StatusUnauthorized)
				return false
			}

			log.Debug(username + " does not exist yet, creating it")

			externalAuthUser := db.UserWithPwd{
				User: db.User{
					Username: username,
					Created:  time.Now(),
					Name:     username,
					Email:    username + "@example.org", // FIXME
					External: true,
					Alert:    false,
				},
			}

			user, err = helpers.Store(r).CreateUser(externalAuthUser)
			if err != nil {
				log.Warn(externalAuthUser.User.Username + " is not created: " + err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return false
			}
		}

		userID = user.ID
		break

	case authUnknown:
		w.WriteHeader(http.StatusUnauthorized)
		return false
	}

	if util.Config.DemoMode {
		if !user.Admin && r.Method != "GET" &&
			!strings.HasSuffix(r.URL.Path, "/tasks") &&
			!strings.HasSuffix(r.URL.Path, "/stop") {
			w.WriteHeader(http.StatusUnauthorized)
			return false
		}
	}

	context.Set(r, "user", &user)
	return true
}

// nolint: gocyclo
func authentication(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ok := authenticationHandler(w, r)
		if ok {
			next.ServeHTTP(w, r)
		}
	})
}

// nolint: gocyclo
func authenticationWithStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store := helpers.Store(r)

		var ok bool

		db.StoreSession(store, r.URL.String(), func() {
			ok = authenticationHandler(w, r)
		})

		if ok {
			next.ServeHTTP(w, r)
		}
	})
}

func fetchUser(store db.Store, userID int) (db.User, error) {
	user, err := store.GetUser(userID)
	if err != nil {
		if err != db.ErrNotFound {
			// internal error
			log.Error(err)
		}
	}
	return user, err
}
