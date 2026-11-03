// Package chi is a type-only offline fixture. It is never executed by analysis.
package chi

import "net/http"

type Router interface {
	Handle(string, http.Handler)
	Get(string, http.HandlerFunc)
	Post(string, http.HandlerFunc)
	Use(...func(http.Handler) http.Handler)
	Route(string, func(Router))
	Mount(string, http.Handler)
	ServeHTTP(http.ResponseWriter, *http.Request)
}

func NewRouter() Router { panic("analysis must never execute fixture code") }
