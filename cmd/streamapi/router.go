package main

import (
	"killfeed/httperror"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/rs/zerolog/log"
)

type HTTPHandlerWithErr func(http.ResponseWriter, *http.Request) *httperror.HTTPError

type Router struct {
	*chi.Mux
}

func NewRouter() *Router {
	return &Router{
		Mux: chi.NewMux(),
	}
}

func (rt *Router) handler(handlerFn HTTPHandlerWithErr) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := handlerFn(w, r); err != nil {
			render.Render(w, r, err)
			log.Warn().Err(err).Int("status", err.Code).Msg("request failed")
		}
	}
}

func (rt *Router) Get(pattern string, handlerFn HTTPHandlerWithErr) {
	rt.Mux.Get(pattern, rt.handler(handlerFn))
}

func (rt *Router) Post(pattern string, handlerFn HTTPHandlerWithErr) {
	rt.Mux.Post(pattern, rt.handler(handlerFn))
}

func (rt *Router) Put(pattern string, handlerFn HTTPHandlerWithErr) {
	rt.Mux.Put(pattern, rt.handler(handlerFn))
}

func (rt *Router) Delete(pattern string, handlerFn HTTPHandlerWithErr) {
	rt.Mux.Delete(pattern, rt.handler(handlerFn))
}
