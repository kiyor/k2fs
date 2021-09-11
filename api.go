package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	kfs "github.com/kiyor/kfs/lib"
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
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Println(err)
	}
	w.Header().Add("content-type", "application/json")
	w.Write(b)
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
	Hash     string
	Size     int64
	SizeH    string
	IsDir    bool
	ModTime  time.Time
	ModTimeH string

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
	q := r.URL.Query()
	action := q.Get("action")
	switch action {
	case "list":
		apiList(w, r)
	case "session":
		apiSession(w, r)
	case "operation":
		apiOperation(w, r)
	default:
		w.Write([]byte("api ok"))
	}
}

func dirSize(path string) (int64, error) {
	key := "size:" + path
	if val, err := cache.Get(key); err == nil {
		return val.(int64), nil
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
	cache.Set(key, size)
	return size, err
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
