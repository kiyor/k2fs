package main

import (
	_ "embed"
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"text/template"
	"time"

	_ "net/http/pprof"

	"github.com/NYTimes/gziphandler"
	"github.com/gorilla/mux"
	"github.com/kiyor/k2fs/lib"
	myhttp "github.com/kiyor/k2fs/pkg/http"
	"golang.org/x/net/webdav"
)

var (
	addr               string
	rootDir, dbDir     string
	intf               string
	port               string
	flagHost           string
	flagStaticFileHost string

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
	flag.StringVar(&dbDir, "db", ".", "db dir")
	flag.StringVar(&flagHost, "host", "", "host if need overwrite; syntax like http://a.com(:8080)")
	flag.StringVar(&flagStaticFileHost, "static", "", "static file host like http://a.com(:8080)")
	flag.StringVar(&metaHost, "meta", "10.43.1.10", "meta host")
	flag.Var(&flagDf, "df", "monitor mount dir")
}

var (
	reIpad  = regexp.MustCompile(` Version/\d+\.\d+`)
	rePhone = regexp.MustCompile(`(P|p)hone`)
	reIos   = regexp.MustCompile(`\((iPhone|iPad);`)
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
	m["ios"] = reIos.MatchString(r.Header.Get("User-Agent"))
	m["phone"] = rePhone.MatchString(r.Header.Get("User-Agent"))
	m["metahost"] = metaHost
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

var metaV2 *lib.MetaV2

func init() {
}

func main() {
	flag.Parse()
	lib.InitRedisPool()
	if rootDir == "." {
		rootDir, _ = os.Getwd()
	}
	if dbDir == "." {
		dbDir = rootDir
	}
	metaV2 = lib.NewMetaV2(rootDir, dbDir)
	// cache = gcache.New(cacheMax).LRU().Build()
	addr = intf + port
	Trash = filepath.Join(rootDir, ".Trash")
	if _, err := os.Stat(Trash); err != nil {
		os.Mkdir(Trash, 0755)
	}
	runtime.GOMAXPROCS(runtime.NumCPU())
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	go func() {
		for {
			t1 := time.Now()
			log.Println("start index")
			metaV2.Index()
			log.Println("index done", time.Since(t1))
			t2 := time.Now()
			metaV2.RemoveOrphan()
			log.Println("remove orphan done", time.Since(t2))
			t3 := time.Now()
			err := metaV2.CacheSize()
			if err != nil {
				log.Println(err)
			}
			log.Println("cache size done", time.Since(t3))
			time.Sleep(55 * time.Minute)
		}
	}()
	go func() {
		for {
			log.Printf("LEN: %d; HIT: %.2f; COUNT: %d", lib.Cache.Len(true), lib.Cache.HitRate()*100, lib.Cache.LookupCount())
			time.Sleep(5 * time.Minute)
		}

	}()

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
	fileServerMain := myhttp.FileServer(myhttp.Dir(rootDir))
	local := http.FileServer(http.Dir("./local"))

	davHandler := &webdav.Handler{
		Prefix:     "/",
		FileSystem: webdav.Dir(rootDir), // Specify the directory to serve
		LockSystem: webdav.NewMemLS(),
	}

	r.PathPrefix("/api").HandlerFunc(api)
	r.PathPrefix("/statics").Handler(http.StripPrefix("/statics", fileServerMain))
	r.PathPrefix("/.local").Handler(http.StripPrefix("/.local", local))
	r.PathPrefix("/photo").HandlerFunc(renderPhoto)
	r.Path("/player").HandlerFunc(renderPlayer)
	r.PathPrefix("/webdav").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "OPTIONS" {
			w.Header().Add("DAV", "1,2")
			w.Header().Add("Allow", "OPTIONS, GET, HEAD, POST, DELETE, PROPFIND, PROPPATCH, COPY, MOVE, LOCK, UNLOCK")
			w.WriteHeader(http.StatusOK)
			return
		}
		davHandler.ServeHTTP(w, r)
	})
	r.PathPrefix("/s").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		path = strings.TrimPrefix(path, "/s/")
		shortMu.Lock()
		defer shortMu.Unlock()
		if short, ok := shortUrl[path]; ok {
			if len(flagStaticFileHost) > 0 {
				short = flagStaticFileHost + short
			}
			http.Redirect(w, r, short, http.StatusFound)
		} else {
			http.NotFound(w, r)
		}
	})
	r.PathPrefix("/").HandlerFunc(universal)
	handler := NewLogHandler().Handler(r)
	handler = gziphandler.GzipHandler(handler)
	http.Handle("/", handler)
	log.Fatal(http.ListenAndServe(addr, nil))
}
