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
	myhttp "github.com/kiyor/k2fs/pkg/http"
)

var (
	addr     string
	rootDir  string
	intf     string
	port     string
	flagHost string

	//go:embed tmpl.html
	basicView string

	//go:embed app.js
	appjs string

	flagDf flagSliceString
)

const (
	// APP name
	APP = "k2fs"
)

func init() {
	flag.StringVar(&intf, "i", "0.0.0.0", "http service interface address")
	flag.StringVar(&port, "l", ":8080", "http service listen port")
	flag.StringVar(&rootDir, "root", ".", "root dir")
	flag.StringVar(&flagHost, "host", "", "host if need overwrite; syntax like http://a.com(:8080)")
	flag.Var(&flagDf, "df", "monitor mount dir")
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
	m := make(map[string]interface{})
	u := scheme + "://" + r.Host
	if len(flagHost) > 0 {
		u = flagHost
	}
	m["host"] = u
	// 	usage := DiskSize([]string(flagDf))
	// 	m["usage"] = usage
	var t *template.Template
	// 	if _, err := os.Stat("tmpl.html"); err == nil {
	// 		t, _ = template.New("").Delims("[[", "]]").ParseFiles("tmpl.html")
	// 	} else {
	t, _ = template.New("").Delims("[[", "]]").Funcs(
		template.FuncMap{
			"slice": genSlice,
		},
	).Parse(basicView)
	// 	}
	t.Execute(w, m)
}

func genSlice(i ...interface{}) chan interface{} {
	o := make(chan interface{})
	go func() {
		for _, v := range i {
			o <- v
		}
		close(o)
	}()
	return o
}

func main() {
	flag.Parse()
	if rootDir == "." {
		rootDir, _ = os.Getwd()
	}
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
	fileServer := myhttp.FileServer(myhttp.Dir(rootDir))
	r.PathPrefix("/api").HandlerFunc(api)
	r.PathPrefix("/statics").Handler(http.StripPrefix("/statics", fileServer))
	r.PathPrefix("/photo").HandlerFunc(renderPhoto)
	r.PathPrefix("/").HandlerFunc(universal)
	handler := NewLogHandler().Handler(r)
	handler = gziphandler.GzipHandler(handler)
	http.Handle("/", handler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
