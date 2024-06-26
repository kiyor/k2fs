package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	golib "github.com/kiyor/golib"
	"github.com/kiyor/k2fs/lib"
	kfs "github.com/kiyor/k2fs/lib"
)

type Resp struct {
	Code int
	Data interface{}
}

func NewResp(w http.ResponseWriter, data interface{}, code ...int) []byte {
	c := 0
	if len(code) > 0 {
		c = code[0]
	}
	r := &Resp{
		Code: c,
		Data: data,
	}
	// 	b, err := json.MarshalIndent(r, "", "  ")
	b, err := json.Marshal(r)
	if err != nil {
		log.Println(err)
	}
	w.Header().Add("Content-Type", "application/json")
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,OPTIONS")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Write(b)
	return b
}

func NewCacheResp(w http.ResponseWriter, data interface{}, cacheKey string, expire time.Duration, code ...int) []byte {
	c := 0
	if len(code) > 0 {
		c = code[0]
	}
	r := &Resp{
		Code: c,
		Data: data,
	}
	// 	b, err := json.MarshalIndent(r, "", "  ")
	b, err := json.Marshal(r)
	if err != nil {
		log.Println(err)
	}
	w.Header().Add("content-type", "application/json")
	w.Write(b)
	lib.Cache.SetWithExpire(cacheKey, b, expire)
	return b
}

func NewErrResp(w http.ResponseWriter, code int, err error) []byte {
	return NewResp(w, err.Error(), code)
}

type Dir struct {
	Dir   string
	UpDir string
	Hash  string
	Files Files
}

func NewDir() *Dir {
	var files Files
	return &Dir{
		Files: files,
	}
}

type File struct {
	Name     string
	Path     string
	Hash     string
	Size     int64
	SizeH    string
	IsDir    bool
	IsImage  bool
	ModTime  time.Time
	ModTimeH string

	ShortCut string

	ThumbLink   string
	Description string
	Tags        []string

	Meta kfs.MetaInfo
}

func NewFile(name string) *File {
	return &File{
		Name: name,
	}
}

type Files []*File

func toJSON(i interface{}) string {
	b, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		log.Println(err)
	}
	return string(b)
}

func api(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		w.Header().Add("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,OPTIONS")
		w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
		w.WriteHeader(200)
		return
	}

	q := r.URL.Query()
	action := q.Get("action")
	switch action {
	case "list":
		apiList(w, r)
	case "thumb":
		apiThumb(w, r)
	case "session":
		apiSession(w, r)
	case "operation":
		apiOperation(w, r)
	case "df":
		apiDf(w, r)
	default:
		w.Write([]byte("api ok"))
	}
}

func apiDf(w http.ResponseWriter, r *http.Request) {
	du := DiskSize([]string(flagDf))
	w.Write([]byte(toJSON(du)))
}

var sizeManager = golib.NewManager(1, 100000)
var sizeTasks = make(chan golib.Task)

var titleManager = golib.NewManager(1, 100000)
var titleTasks = make(chan golib.Task)

func init() {
	sizeManager.Start(sizeTasks)
	titleManager.Start(titleTasks)
}

func fetchTitle(path string) (string, error) {
	key := "title:" + path
	titleTasks <- golib.NewTask(
		func() error {
			if _, err := lib.Cache.Get(key); err == nil {
				return nil
			} else {
				if val, err := metaV2.Get(path); err == nil {
					ctx := val.GetContext()
					if ctx != nil && ctx["Title"] != nil {
						lib.Cache.SetWithExpire(key, ctx["Title"].(string), 24*time.Hour)
					} else {
						name := strings.Trim(path, "/")
						name = filepath.Base(name)
						name, _ = isSearchable(name)
						res, err := lib.NewSearchClient().Search(name)
						if err == nil {
							if ctx == nil {
								ctx = make(map[string]interface{})
							}
							ctx["Title"] = res.Title
							val.SetContext(ctx)
							metaV2.Set(val)
							lib.Cache.SetWithExpire(key, res.Title, 24*time.Hour)
						} else {
							log.Println(err)
						}
					}
				}
				// log.Println("fetch title", key)
			}
			return nil
		},
		nil,
		false,
	)
	return "", nil
}

func dirSize2(path string) (int64, error) {
	path = strings.TrimLeft(path, "/")
	key := "size:" + path
	if val, err := lib.Cache.Get(key); err == nil {
		return int64(val.(float64)), nil
	}
	sizeTasks <- golib.NewTask(
		func() error {
			if _, err := lib.Cache.Get(key); err == nil {
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			size, err := metaV2.SizeWithTimeout(path, ctx)
			if err != nil {
				return err
			}
			lib.Cache.SetWithExpire(key, size, time.Hour)
			return nil
		},
		nil,
		false,
	)
	return 1, nil
}

func dirSize(path string) (int64, error) {
	key := "size:" + path
	if val, err := lib.Cache.Get(key); err == nil {
		return val.(int64), nil
	}
	sizeTasks <- golib.NewTask(
		func() error {
			if _, err := lib.Cache.Get(key); err == nil {
				return nil
			}
			var size int64
			err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					size += info.Size()
				}
				return err
			})
			if err == nil {
				lib.Cache.SetWithExpire(key, size, time.Hour)
			}
			return err
		},
		nil,
		false,
	)
	return 1, nil
}

func prettyTime(t time.Time) string {
	since := time.Since(t)
	switch {
	case since < (1 * time.Second):
		return "1s"

	case since < (60 * time.Second):
		s := strings.Split(fmt.Sprint(since), ".")[0]
		return s + "s"

	case since < (60 * time.Minute):
		s := strings.Split(fmt.Sprint(since), ".")[0]
		return strings.Split(s, "m")[0] + "m"

	case since < (24 * time.Hour):
		s := strings.Split(fmt.Sprint(since), ".")[0]
		return strings.Split(s, "h")[0] + "h"

	default:
		return t.Format("01-02-06")
	}
}
