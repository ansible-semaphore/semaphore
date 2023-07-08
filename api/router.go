package api

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/ansible-semaphore/semaphore/api/helpers"
	"github.com/ansible-semaphore/semaphore/db"

	"github.com/ansible-semaphore/semaphore/api/projects"
	"github.com/ansible-semaphore/semaphore/api/sockets"
	"github.com/ansible-semaphore/semaphore/util"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"
)

var publicAssets2 = packr.NewBox("../web/dist")

func StoreMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		store := helpers.Store(r)
		var url = r.URL.String()

		db.StoreSession(store, url, func() {
			next.ServeHTTP(w, r)
		})
	})
}

// JSONMiddleware ensures that all the routes respond with Json, this is added by default to all routes
func JSONMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// plainTextMiddleware resets headers to Plain Text if needed
func plainTextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("content-type", "text/plain; charset=utf-8")
		next.ServeHTTP(w, r)
	})
}

func pongHandler(w http.ResponseWriter, r *http.Request) {
	//nolint: errcheck
	w.Write([]byte("pong"))
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.WriteHeader(http.StatusNotFound)
	//nolint: errcheck
	w.Write([]byte("404 not found"))
	fmt.Println(r.Method, ":", r.URL.String(), "--> 404 Not Found")
}

// Route declares all routes
func Route() *mux.Router {
	r := mux.NewRouter()
	r.NotFoundHandler = http.HandlerFunc(servePublic)

	webPath := "/"
	if util.WebHostURL != nil {
		webPath = util.WebHostURL.Path
		if !strings.HasSuffix(webPath, "/") {
			webPath += "/"
		}
	}

	r.Use(mux.CORSMethodMiddleware(r))

	pingRouter := r.Path(webPath + "api/ping").Subrouter()
	pingRouter.Use(plainTextMiddleware)
	pingRouter.Methods("GET", "HEAD").HandlerFunc(pongHandler)

	publicAPIRouter := r.PathPrefix(webPath + "api").Subrouter()

	publicAPIRouter.Use(StoreMiddleware, JSONMiddleware)

	publicAPIRouter.HandleFunc("/auth/login", login).Methods("POST")
	publicAPIRouter.HandleFunc("/auth/logout", logout).Methods("POST")

	authenticatedWS := r.PathPrefix(webPath + "api").Subrouter()
	authenticatedWS.Use(JSONMiddleware, authenticationWithStore)
	authenticatedWS.Path("/ws").HandlerFunc(sockets.Handler).Methods("GET", "HEAD")

	authenticatedAPI := r.PathPrefix(webPath + "api").Subrouter()

	authenticatedAPI.Use(StoreMiddleware, JSONMiddleware, authentication)

	authenticatedAPI.Path("/info").HandlerFunc(getSystemInfo).Methods("GET", "HEAD")

	authenticatedAPI.Path("/projects").HandlerFunc(projects.GetProjects).Methods("GET", "HEAD")
	authenticatedAPI.Path("/projects").HandlerFunc(projects.AddProject).Methods("POST")
	authenticatedAPI.Path("/events").HandlerFunc(getAllEvents).Methods("GET", "HEAD")
	authenticatedAPI.HandleFunc("/events/last", getLastEvents).Methods("GET", "HEAD")

	authenticatedAPI.Path("/users").HandlerFunc(getUsers).Methods("GET", "HEAD")
	authenticatedAPI.Path("/users").HandlerFunc(addUser).Methods("POST")
	authenticatedAPI.Path("/user").HandlerFunc(getUser).Methods("GET", "HEAD")

	tokenAPI := authenticatedAPI.PathPrefix("/user").Subrouter()
	tokenAPI.Path("/tokens").HandlerFunc(getAPITokens).Methods("GET", "HEAD")
	tokenAPI.Path("/tokens").HandlerFunc(createAPIToken).Methods("POST")
	tokenAPI.HandleFunc("/tokens/{token_id}", expireAPIToken).Methods("DELETE")

	userAPI := authenticatedAPI.Path("/users/{user_id}").Subrouter()
	userAPI.Use(getUserMiddleware)

	userAPI.Methods("GET", "HEAD").HandlerFunc(getUser)
	userAPI.Methods("PUT").HandlerFunc(updateUser)
	userAPI.Methods("DELETE").HandlerFunc(deleteUser)

	userPasswordAPI := authenticatedAPI.PathPrefix("/users/{user_id}").Subrouter()
	userPasswordAPI.Use(getUserMiddleware)
	userPasswordAPI.Path("/password").HandlerFunc(updateUserPassword).Methods("POST")

	projectGet := authenticatedAPI.Path("/project/{project_id}").Subrouter()
	projectGet.Use(projects.ProjectMiddleware)
	projectGet.Methods("GET", "HEAD").HandlerFunc(projects.GetProject)

	projectUserAPI := authenticatedAPI.PathPrefix("/project/{project_id}").Subrouter()
	projectUserAPI.Use(projects.ProjectMiddleware)

	projectUserAPI.Path("/events").HandlerFunc(getAllEvents).Methods("GET", "HEAD")
	projectUserAPI.HandleFunc("/events/last", getLastEvents).Methods("GET", "HEAD")

	projectUserAPI.Path("/users").HandlerFunc(projects.GetUsers).Methods("GET", "HEAD")

	projectUserAPI.Path("/keys").HandlerFunc(projects.GetKeys).Methods("GET", "HEAD")
	projectUserAPI.Path("/keys").HandlerFunc(projects.AddKey).Methods("POST")

	projectUserAPI.Path("/repositories").HandlerFunc(projects.GetRepositories).Methods("GET", "HEAD")
	projectUserAPI.Path("/repositories").HandlerFunc(projects.AddRepository).Methods("POST")

	projectUserAPI.Path("/inventory").HandlerFunc(projects.GetInventory).Methods("GET", "HEAD")
	projectUserAPI.Path("/inventory").HandlerFunc(projects.AddInventory).Methods("POST")

	projectUserAPI.Path("/environment").HandlerFunc(projects.GetEnvironment).Methods("GET", "HEAD")
	projectUserAPI.Path("/environment").HandlerFunc(projects.AddEnvironment).Methods("POST")

	projectUserAPI.Path("/tasks").HandlerFunc(projects.GetAllTasks).Methods("GET", "HEAD")
	projectUserAPI.HandleFunc("/tasks/last", projects.GetLastTasks).Methods("GET", "HEAD")
	projectUserAPI.Path("/tasks").HandlerFunc(projects.AddTask).Methods("POST")

	projectUserAPI.Path("/templates").HandlerFunc(projects.GetTemplates).Methods("GET", "HEAD")
	projectUserAPI.Path("/templates").HandlerFunc(projects.AddTemplate).Methods("POST")

	projectUserAPI.Path("/schedules").HandlerFunc(projects.AddSchedule).Methods("POST")
	projectUserAPI.Path("/schedules/validate").HandlerFunc(projects.ValidateScheduleCronFormat).Methods("POST")

	projectUserAPI.Path("/views").HandlerFunc(projects.GetViews).Methods("GET", "HEAD")
	projectUserAPI.Path("/views").HandlerFunc(projects.AddView).Methods("POST")
	projectUserAPI.Path("/views/positions").HandlerFunc(projects.SetViewPositions).Methods("POST")

	projectAdminAPI := authenticatedAPI.Path("/project/{project_id}").Subrouter()
	projectAdminAPI.Use(projects.ProjectMiddleware, projects.MustBeAdmin)
	projectAdminAPI.Methods("PUT").HandlerFunc(projects.UpdateProject)
	projectAdminAPI.Methods("DELETE").HandlerFunc(projects.DeleteProject)

	projectAdminUsersAPI := authenticatedAPI.PathPrefix("/project/{project_id}").Subrouter()
	projectAdminUsersAPI.Use(projects.ProjectMiddleware, projects.MustBeAdmin)
	projectAdminUsersAPI.Path("/users").HandlerFunc(projects.AddUser).Methods("POST")

	projectUserManagement := projectAdminUsersAPI.PathPrefix("/users").Subrouter()
	projectUserManagement.Use(projects.UserMiddleware)

	projectUserManagement.HandleFunc("/{user_id}", projects.GetUsers).Methods("GET", "HEAD")
	projectUserManagement.HandleFunc("/{user_id}/admin", projects.MakeUserAdmin).Methods("POST")
	projectUserManagement.HandleFunc("/{user_id}/admin", projects.MakeUserAdmin).Methods("DELETE")
	projectUserManagement.HandleFunc("/{user_id}", projects.RemoveUser).Methods("DELETE")

	projectKeyManagement := projectUserAPI.PathPrefix("/keys").Subrouter()
	projectKeyManagement.Use(projects.KeyMiddleware)

	projectKeyManagement.HandleFunc("/{key_id}", projects.GetKeys).Methods("GET", "HEAD")
	projectKeyManagement.HandleFunc("/{key_id}/refs", projects.GetKeyRefs).Methods("GET", "HEAD")
	projectKeyManagement.HandleFunc("/{key_id}", projects.UpdateKey).Methods("PUT")
	projectKeyManagement.HandleFunc("/{key_id}", projects.RemoveKey).Methods("DELETE")

	projectRepoManagement := projectUserAPI.PathPrefix("/repositories").Subrouter()
	projectRepoManagement.Use(projects.RepositoryMiddleware)

	projectRepoManagement.HandleFunc("/{repository_id}", projects.GetRepositories).Methods("GET", "HEAD")
	projectRepoManagement.HandleFunc("/{repository_id}/refs", projects.GetRepositoryRefs).Methods("GET", "HEAD")
	projectRepoManagement.HandleFunc("/{repository_id}", projects.UpdateRepository).Methods("PUT")
	projectRepoManagement.HandleFunc("/{repository_id}", projects.RemoveRepository).Methods("DELETE")

	projectInventoryManagement := projectUserAPI.PathPrefix("/inventory").Subrouter()
	projectInventoryManagement.Use(projects.InventoryMiddleware)

	projectInventoryManagement.HandleFunc("/{inventory_id}", projects.GetInventory).Methods("GET", "HEAD")
	projectInventoryManagement.HandleFunc("/{inventory_id}/refs", projects.GetInventoryRefs).Methods("GET", "HEAD")
	projectInventoryManagement.HandleFunc("/{inventory_id}", projects.UpdateInventory).Methods("PUT")
	projectInventoryManagement.HandleFunc("/{inventory_id}", projects.RemoveInventory).Methods("DELETE")

	projectEnvManagement := projectUserAPI.PathPrefix("/environment").Subrouter()
	projectEnvManagement.Use(projects.EnvironmentMiddleware)

	projectEnvManagement.HandleFunc("/{environment_id}", projects.GetEnvironment).Methods("GET", "HEAD")
	projectEnvManagement.HandleFunc("/{environment_id}/refs", projects.GetEnvironmentRefs).Methods("GET", "HEAD")
	projectEnvManagement.HandleFunc("/{environment_id}", projects.UpdateEnvironment).Methods("PUT")
	projectEnvManagement.HandleFunc("/{environment_id}", projects.RemoveEnvironment).Methods("DELETE")

	projectTmplManagement := projectUserAPI.PathPrefix("/templates").Subrouter()
	projectTmplManagement.Use(projects.TemplatesMiddleware)

	projectTmplManagement.HandleFunc("/{template_id}", projects.UpdateTemplate).Methods("PUT")
	projectTmplManagement.HandleFunc("/{template_id}", projects.RemoveTemplate).Methods("DELETE")
	projectTmplManagement.HandleFunc("/{template_id}", projects.GetTemplate).Methods("GET")
	projectTmplManagement.HandleFunc("/{template_id}/refs", projects.GetTemplateRefs).Methods("GET", "HEAD")
	projectTmplManagement.HandleFunc("/{template_id}/tasks", projects.GetAllTasks).Methods("GET")
	projectTmplManagement.HandleFunc("/{template_id}/tasks/last", projects.GetLastTasks).Methods("GET")
	projectTmplManagement.HandleFunc("/{template_id}/schedules", projects.GetTemplateSchedules).Methods("GET")

	projectTaskManagement := projectUserAPI.PathPrefix("/tasks").Subrouter()
	projectTaskManagement.Use(projects.GetTaskMiddleware)

	projectTaskManagement.HandleFunc("/{task_id}/output", projects.GetTaskOutput).Methods("GET", "HEAD")
	projectTaskManagement.HandleFunc("/{task_id}", projects.GetTask).Methods("GET", "HEAD")
	projectTaskManagement.HandleFunc("/{task_id}", projects.RemoveTask).Methods("DELETE")
	projectTaskManagement.HandleFunc("/{task_id}/stop", projects.StopTask).Methods("POST")

	projectScheduleManagement := projectUserAPI.PathPrefix("/schedules").Subrouter()
	projectScheduleManagement.Use(projects.SchedulesMiddleware)
	projectScheduleManagement.HandleFunc("/{schedule_id}", projects.GetSchedule).Methods("GET", "HEAD")
	projectScheduleManagement.HandleFunc("/{schedule_id}", projects.UpdateSchedule).Methods("PUT")
	projectScheduleManagement.HandleFunc("/{schedule_id}", projects.RemoveSchedule).Methods("DELETE")

	projectViewManagement := projectUserAPI.PathPrefix("/views").Subrouter()
	projectViewManagement.Use(projects.ViewMiddleware)
	projectViewManagement.HandleFunc("/{view_id}", projects.GetViews).Methods("GET", "HEAD")
	projectViewManagement.HandleFunc("/{view_id}", projects.UpdateView).Methods("PUT")
	projectViewManagement.HandleFunc("/{view_id}", projects.RemoveView).Methods("DELETE")
	projectViewManagement.HandleFunc("/{view_id}/templates", projects.GetViewTemplates).Methods("GET", "HEAD")

	if os.Getenv("DEBUG") == "1" {
		defer debugPrintRoutes(r)
	}

	return r
}

func debugPrintRoutes(r *mux.Router) {
	err := r.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
		pathTemplate, err := route.GetPathTemplate()
		if err == nil {
			fmt.Println("ROUTE:", pathTemplate)
		}
		pathRegexp, err := route.GetPathRegexp()
		if err == nil {
			fmt.Println("Path regexp:", pathRegexp)
		}
		queriesTemplates, err := route.GetQueriesTemplates()
		if err == nil {
			fmt.Println("Queries templates:", strings.Join(queriesTemplates, ","))
		}
		queriesRegexps, err := route.GetQueriesRegexp()
		if err == nil {
			fmt.Println("Queries regexps:", strings.Join(queriesRegexps, ","))
		}
		methods, err := route.GetMethods()
		if err == nil {
			fmt.Println("Methods:", strings.Join(methods, ","))
		}
		fmt.Println()
		return nil
	})

	if err != nil {
		fmt.Println(err)
	}
}

// nolint: gocyclo
func servePublic(w http.ResponseWriter, r *http.Request) {
	webPath := "/"
	if util.WebHostURL != nil {
		webPath = util.WebHostURL.RequestURI()
	}

	path := r.URL.Path

	if path == webPath+"api" || strings.HasPrefix(path, webPath+"api/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if !strings.Contains(path, ".") {
		path = "/index.html"
	}

	path = strings.Replace(path, webPath+"/", "", 1)
	split := strings.Split(path, ".")
	suffix := split[len(split)-1]

	var res []byte
	var err error

	res, err = publicAssets2.MustBytes(path)

	if err != nil {
		notFoundHandler(w, r)
		return
	}

	// replace base path
	if util.WebHostURL != nil && path == "/index.html" {
		baseURL := util.WebHostURL.String()
		if !strings.HasSuffix(baseURL, "/") {
			baseURL += "/"
		}
		res = []byte(strings.Replace(string(res),
			"<base href=\"/\">",
			"<base href=\""+baseURL+"\">",
			1))
	}

	contentType := "text/plain"
	switch suffix {
	case "png":
		contentType = "image/png"
	case "jpg", "jpeg":
		contentType = "image/jpeg"
	case "gif":
		contentType = "image/gif"
	case "js":
		contentType = "application/javascript"
	case "css":
		contentType = "text/css"
	case "woff":
		contentType = "application/x-font-woff"
	case "ttf":
		contentType = "application/x-font-ttf"
	case "otf":
		contentType = "application/x-font-otf"
	case "html":
		contentType = "text/html"
	}

	w.Header().Set("content-type", contentType)
	_, err = w.Write(res)
	util.LogWarning(err)
}

func getSystemInfo(w http.ResponseWriter, r *http.Request) {
	body := map[string]interface{}{
		"version": util.Version,
		"ansible": util.AnsibleVersion(),
		"demo":    util.Config.DemoMode,
	}

	helpers.WriteJSON(w, http.StatusOK, body)
}
