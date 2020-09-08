package mux

import "net/http"

type Mux interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
	Handler(r *http.Request) (http.Handler, string)
	Handle(pattern string, handler http.Handler)
}