package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/a-h/templ"
)

// compToString takes a templ.Component and renders it to a string
// this is useful for sending SSE updates to the client, where we need to send the rendered HTML as a string
func compToString(component templ.Component) (string, error) {
	var buf strings.Builder
	err := component.Render(context.TODO(), &buf)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// render takes a templ.Component and writes it to the http.ResponseWriter for HTML responses
func render(w http.ResponseWriter, r *http.Request, component templ.Component) {
	// headers must be set before WriteHeader, otherwise they are ignored
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	component.Render(r.Context(), w)
}
