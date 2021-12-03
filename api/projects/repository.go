package projects

import (
	log "github.com/Sirupsen/logrus"
	"github.com/neo1908/semaphore/api/helpers"
	"github.com/neo1908/semaphore/db"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/neo1908/semaphore/util"
	"github.com/gorilla/context"
)

func removeAllByPattern(path string, filenamePattern string) error {
	d, err := os.Open(path)
	if err != nil {
		return err
	}
	defer d.Close()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		if matched, _ := filepath.Match(filenamePattern, name); !matched {
			continue
		}
		if err := os.RemoveAll(filepath.Join(path, name)); err != nil {
			return err
		}
	}
	return nil
}

func clearRepositoryCache(repository db.Repository) error {
	return removeAllByPattern(util.Config.TmpPath, "repository_"+strconv.Itoa(repository.ID)+"_*")
}

// RepositoryMiddleware ensures a repository exists and loads it to the context
func RepositoryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		project := context.Get(r, "project").(db.Project)
		repositoryID, err := helpers.GetIntParam("repository_id", w, r)
		if err != nil {
			return
		}

		repository, err := helpers.Store(r).GetRepository(project.ID, repositoryID)

		if err != nil {
			helpers.WriteError(w, err)
			return
		}

		context.Set(r, "repository", repository)
		next.ServeHTTP(w, r)
	})
}

// GetRepositories returns all repositories in a project sorted by type
func GetRepositories(w http.ResponseWriter, r *http.Request) {
	if repo := context.Get(r, "repository"); repo != nil {
		helpers.WriteJSON(w, http.StatusOK, repo.(db.Repository))
		return
	}

	project := context.Get(r, "project").(db.Project)

	repos, err := helpers.Store(r).GetRepositories(project.ID, helpers.QueryParams(r.URL))

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	helpers.WriteJSON(w, http.StatusOK, repos)
}

// AddRepository creates a new repository in the database
func AddRepository(w http.ResponseWriter, r *http.Request) {
	project := context.Get(r, "project").(db.Project)

	var repository db.Repository

	if !helpers.Bind(w, r, &repository) {
		return
	}

	if repository.ProjectID != project.ID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Project ID in body and URL must be the same",
		})
	}

	newRepo, err := helpers.Store(r).CreateRepository(repository)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	user := context.Get(r, "user").(*db.User)

	objType := db.EventRepository

	desc := "Repository (" + repository.GitURL + ") created"
	_, err = helpers.Store(r).CreateEvent(db.Event{
		UserID:      &user.ID,
		ProjectID:   &newRepo.ProjectID,
		ObjectType:  &objType,
		ObjectID:    &newRepo.ID,
		Description: &desc,
	})

	if err != nil {
		log.Error(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// UpdateRepository updates the values of a repository in the database
func UpdateRepository(w http.ResponseWriter, r *http.Request) {
	oldRepo := context.Get(r, "repository").(db.Repository)
	var repository db.Repository

	if !helpers.Bind(w, r, &repository) {
		return
	}

	if repository.ID != oldRepo.ID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Repository ID in body and URL must be the same",
		})
		return
	}

	if repository.ProjectID != oldRepo.ProjectID {
		helpers.WriteJSON(w, http.StatusBadRequest, map[string]string{
			"error": "Project ID in body and URL must be the same",
		})
		return
	}

	err := helpers.Store(r).UpdateRepository(repository)

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	if oldRepo.GitURL != repository.GitURL {
		util.LogWarning(clearRepositoryCache(oldRepo))
	}

	user := context.Get(r, "user").(*db.User)

	desc := "Repository (" + repository.GitURL + ") updated"
	objType := db.EventRepository

	_, err = helpers.Store(r).CreateEvent(db.Event{
		UserID:      &user.ID,
		ProjectID:   &repository.ProjectID,
		Description: &desc,
		ObjectID:    &repository.ID,
		ObjectType:  &objType,
	})

	if err != nil {
		log.Error(err)
	}

	w.WriteHeader(http.StatusNoContent)
}

// RemoveRepository deletes a repository from a project in the database
func RemoveRepository(w http.ResponseWriter, r *http.Request) {
	repository := context.Get(r, "repository").(db.Repository)

	var err error

	softDeletion := r.URL.Query().Get("setRemoved") == "1"

	if softDeletion {
		err = helpers.Store(r).DeleteRepositorySoft(repository.ProjectID, repository.ID)
	} else {
		err = helpers.Store(r).DeleteRepository(repository.ProjectID, repository.ID)
		if err == db.ErrInvalidOperation {
			helpers.WriteJSON(w, http.StatusBadRequest, map[string]interface{}{
				"error": "Repository is in use by one or more templates",
				"inUse": true,
			})
			return
		}
	}

	if err != nil {
		helpers.WriteError(w, err)
		return
	}

	util.LogWarning(clearRepositoryCache(repository))
	user := context.Get(r, "user").(*db.User)

	desc := "Repository (" + repository.GitURL + ") deleted"
	_, err = helpers.Store(r).CreateEvent(db.Event{
		UserID:      &user.ID,
		ProjectID:   &repository.ProjectID,
		Description: &desc,
	})

	if err != nil {
		log.Error(err)
	}

	w.WriteHeader(http.StatusNoContent)
}
