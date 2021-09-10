package main

import (
	_ "embed"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	photoExt = []string{".jpg", ".png", ".gif", ".jpeg"}

	//go:embed photo.html
	photoTmpl string
)

type Image template.HTML

type Images struct {
	Title  string
	Images []Image
}

func (images Images) List() template.JS {
	var out string
	for _, v_ := range images.Images {
		v := string(v_)
		s := (&url.URL{Path: v}).String()
		out += fmt.Sprintf("{url:\"%s\"},", s)
	}
	if len(out) > 0 {
		return template.JS(out[:len(out)-1])
	}
	return ""
}

func imagesList(images Images) template.JS {
	return images.List()
}

func renderPhoto(w http.ResponseWriter, r *http.Request) {
	// trim /photo/test -> test
	path := r.URL.Path[len("/photo/"):]
	dir := filepath.Join(rootDir, path)

	var images Images

	images.Title = filepath.Base(dir)
	fs := readDir(dir)
	if len(fs) == 0 {
		http.Redirect(w, r, r.URL.String(), 302)
		return
	}
	sort.Slice(fs, func(i, j int) bool {
		return fs[i].Name() < fs[j].Name()
	})
	for _, f := range fs {
		fp := filepath.Join("/statics", path, f.Name())
		images.Images = append(images.Images, Image(fp))
	}
	f := template.FuncMap{
		"imageslist": imagesList,
	}
	t, err := template.New("index").Funcs(f).Parse(photoTmpl)
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}
	err = t.Execute(w, images)
	if err != nil {
		fmt.Fprintln(w, err.Error())
	}
}

func readDir(path string) (fs []os.FileInfo) {
	files, err := ioutil.ReadDir(path)
	if err != nil {
		log.Println(err)
	}
	for _, f := range files {
		for _, ext := range photoExt {
			if strings.ToLower(filepath.Ext(f.Name())) == ext {
				fs = append(fs, f)
				break
			}
		}
	}
	return
}
