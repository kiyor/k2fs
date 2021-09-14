package main

import (
	_ "embed"
	"flag"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"text/template"

	"github.com/NYTimes/gziphandler"
	"github.com/bluele/gcache"
	"github.com/gorilla/mux"
)

var (
	addr    string
	rootDir string
	intf    string
	port    string

	//go:embed tmpl.html
	basicView string

	//go:embed app.js
	appjs string
)

const (
	// APP name
	APP = "k2fs"
)

func init() {
	flag.StringVar(&intf, "i", "0.0.0.0", "http service interface address")
	flag.StringVar(&port, "l", ":8080", "http service listen port")
	flag.StringVar(&rootDir, "root", ".", "root dir")
}

func universal(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		w.WriteHeader(200)
		return
	}
	w.Header().Add("content-type", "text/html")
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	u := scheme + "://" + r.Host
	var t *template.Template
	// 	if _, err := os.Stat("tmpl.html"); err == nil {
	// 		t, _ = template.New("").Delims("[[", "]]").ParseFiles("tmpl.html")
	// 	} else {
	t, _ = template.New("").Delims("[[", "]]").Parse(basicView)
	// 	}
	t.Execute(w, u)
}

func main() {
	flag.Parse()
	cache = gcache.New(cacheMax).LRU().Expiration(cacheTimeout).Build()
	addr = intf + port
	Trash = filepath.Join(rootDir, ".Trash")
	if _, err := os.Stat(Trash); err != nil {
		os.Mkdir(Trash, 0755)
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	r := mux.NewRouter()
	r.Path("/app.js").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "application/javascript")
		f, err := os.Open("app.js")
		if err != nil {
			w.Write([]byte(appjs))
		} else {
			io.Copy(w, f)
		}
	})
	fileServer := http.FileServer(http.Dir(rootDir))
	r.PathPrefix("/api").HandlerFunc(api)
	r.PathPrefix("/statics").Handler(http.StripPrefix("/statics", fileServer))
	r.PathPrefix("/photo").HandlerFunc(renderPhoto)
	r.PathPrefix("/").HandlerFunc(universal)
	handler := NewLogHandler().Handler(r)
	handler = gziphandler.GzipHandler(handler)
	http.Handle("/", handler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
