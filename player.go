package main

import (
	_ "embed"
	"net/http"
	"text/template"
)

var (
	//go:embed player.html
	playerTmpl string
)

func renderPlayer(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	t := template.Must(template.New("player").Parse(playerTmpl))
	t.Execute(w, query)
}
