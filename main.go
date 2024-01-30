package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"text/template"

	"github.com/NYTimes/gziphandler"
	"github.com/bluele/gcache"
	"github.com/gorilla/mux"
	"github.com/kiyor/k2fs/lib"
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
	//go:embed bootstrap.css
	bootstrapcss string

	metaHost string

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
	flag.StringVar(&metaHost, "meta", "192.168.10.31", "meta host")
	flag.Var(&flagDf, "df", "monitor mount dir")
}

var (
	reIpad  = regexp.MustCompile(` Version/\d+\.\d+`)
	rePhone = regexp.MustCompile(`(P|p)hone`)
)

func req2map(r *http.Request) map[string]interface{} {
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
	m["ios"] = reIpad.MatchString(r.Header.Get("User-Agent"))
	m["phone"] = rePhone.MatchString(r.Header.Get("User-Agent"))
	// 	log.Println("ios:", m["ios"])
	return m
}

func universal(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/favicon.ico" {
		w.WriteHeader(200)
		return
	}
	w.Header().Add("content-type", "text/html")
	var t *template.Template
	t, _ = template.New("").Delims("[[", "]]").Funcs(
		template.FuncMap{
			"slice": genSlice,
		},
	).Parse(basicView)
	t.Execute(w, req2map(r))
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

func init() {
	fmt.Println("A")
	fmt.Println("B")
	fmt.Println("C")
	fmt.Println("D")
	fmt.Println("E")
	fmt.Println("F")
	fmt.Println("G")
	fmt.Println("H")

}

func main() {
	flag.Parse()
	lib.InitRedisPool()
	if rootDir == "." {
		rootDir, _ = os.Getwd()
	}
	cache = gcache.New(cacheMax).LRU().Build()
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
		w.Header().Add("cache-control", "public, max-age=300")
		t, _ := template.New("app.js").Delims("[[", "]]").Funcs(
			template.FuncMap{
				"slice": genSlice,
			},
		).Parse(appjs)
		t.Execute(w, req2map(r))
	})
	r.Path("/bootstrap.css").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("content-type", "text/css")
		w.Header().Add("cache-control", "public, max-age=300")
		w.Write([]byte(bootstrapcss))
	})
	fileServer := myhttp.FileServer(myhttp.Dir(rootDir))
	local := http.FileServer(http.Dir("./local"))
	r.PathPrefix("/api").HandlerFunc(api)
	r.PathPrefix("/statics").Handler(http.StripPrefix("/statics", fileServer))
	r.PathPrefix("/.local").Handler(http.StripPrefix("/.local", local))
	r.PathPrefix("/photo").HandlerFunc(renderPhoto)
	r.PathPrefix("/").HandlerFunc(universal)
	handler := NewLogHandler().Handler(r)
	handler = gziphandler.GzipHandler(handler)
	http.Handle("/", handler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
