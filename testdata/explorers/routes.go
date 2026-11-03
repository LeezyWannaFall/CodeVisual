package explorers

import (
	"github.com/go-chi/chi/v5"
	"net/http"
)

func Routes() chi.Router {
	users := UserRoutes()
	r := chi.NewRouter()
	r.Use(Logging)
	r.Get("/health", Health)
	r.Mount("/api", users)
	r.Route("/admin", func(admin chi.Router) { admin.Post("/refresh", Refresh) })
	return r
}
func UserRoutes() chi.Router                      { r := chi.NewRouter(); r.Get("/users/{id}", ShowUser); return r }
func Health(http.ResponseWriter, *http.Request)   {}
func ShowUser(http.ResponseWriter, *http.Request) {}
func Refresh(http.ResponseWriter, *http.Request)  {}
func Logging(next http.Handler) http.Handler      { return next }
