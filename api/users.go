package api

import (
	"net/http"

	log "github.com/Sirupsen/logrus"
	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/ansible-semaphore/semaphore/db"

	"github.com/ansible-semaphore/semaphore/util"
	"github.com/gorilla/context"
)

func getUsers(w http.ResponseWriter, r *http.Request) {
	users, err := helpers.Store(r).GetUsers(db.RetrieveQueryParams{})

	if err != nil {
		panic(err)
	}

	helpers.WriteJSON(w, http.StatusOK, users)
}

func addUser(w http.ResponseWriter, r *http.Request) {
	var user db.UserWithPwd
	if !helpers.Bind(w, r, &user) {
		return
	}

	editor := context.Get(r, "user").(*db.User)
	if !editor.Admin {
		log.Warn(editor.Username + " is not permitted to create users")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	newUser, err := helpers.Store(r).CreateUser(user)

	if err != nil {
		log.Warn(editor.Username + " is not created: " + err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	helpers.WriteJSON(w, http.StatusCreated, newUser)
}

func getUserMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := helpers.GetIntParam("user_id", w, r)

		if err != nil {
			return
		}

		user, err := helpers.Store(r).GetUser(userID)

		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		editor := context.Get(r, "user").(*db.User)

		if !editor.Admin && editor.ID != user.ID {
			log.Warn(editor.Username + " is not permitted to edit users")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		context.Set(r, "_user", user)
		next.ServeHTTP(w, r)
	})
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	oldUser := context.Get(r, "_user").(db.User)
	editor := context.Get(r, "user").(*db.User)

	var user db.UserWithPwd
	if !helpers.Bind(w, r, &user) {
		return
	}

	if !editor.Admin && editor.ID != oldUser.ID {
		log.Warn(editor.Username + " is not permitted to edit users")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if editor.ID == oldUser.ID && oldUser.Admin != user.Admin {
		log.Warn("User can't edit his own role")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if oldUser.External && oldUser.Username != user.Username {
		log.Warn("Username is not editable for external LDAP users")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	user.ID = oldUser.ID
	if err := helpers.Store(r).UpdateUser(user); err != nil {
		log.Error(err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateUserPassword(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "_user").(db.User)
	editor := context.Get(r, "user").(*db.User)

	var pwd struct {
		Pwd string `json:"password"`
	}

	if !editor.Admin && editor.ID != user.ID {
		log.Warn(editor.Username + " is not permitted to edit users")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if user.External {
		log.Warn("Password is not editable for external LDAP users")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !helpers.Bind(w, r, &pwd) {
		return
	}

	if err := helpers.Store(r).SetUserPassword(user.ID, pwd.Pwd); err != nil {
		util.LogWarning(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, "_user").(db.User)
	editor := context.Get(r, "user").(*db.User)

	if !editor.Admin && editor.ID != user.ID {
		log.Warn(editor.Username + " is not permitted to delete users")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if err := helpers.Store(r).DeleteUser(user.ID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusNoContent)
}
