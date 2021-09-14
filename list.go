package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	kfs "github.com/kiyor/kfs/lib"
)

var hideExt = []string{
	".MHT",
	".CHM",
	".LNK",
	".APK",
	".PNG",
	".TXT",
	".TODO",
	".URL",
	".HTM",
	kfs.KFS,
}
var hideContain = []string{
	"padding_file",
}

func needHide(path string) bool {
	for _, v := range hideExt {
		if strings.ToUpper(filepath.Ext(path)) == v {
			return true
		}
	}
	for _, v := range hideContain {
		if strings.Contains(path, v) {
			return true
		}
	}
	return false
}

func upDir(path string) string {
	if path == "/" {
		return path
	}
	return path[:len(path)-len(filepath.Base(path))]
}

func apiList(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	m := make(map[string]string)
	err := json.NewDecoder(r.Body).Decode(&m)
	if err != nil {
		log.Println(err)
		NewErrResp(w, 1, err)
		return
	}
	// 	path := "." + q.Get("path")
	m["path"] = strings.TrimRight(m["path"], "/")
	if len(m["path"]) == 0 {
		m["path"] = "/"
	}
	if _, ok := m["listdir"]; !ok {
		m["listdir"] = "read"
	}
	path := filepath.Join(rootDir, m["path"])
	f, err := os.Stat(path)
	if err != nil {
		log.Println(err)
		NewErrResp(w, 1, err)
		return
	}
	var isRead, isFind bool
	switch m["listdir"] {
	case "read":
		isRead = true
	case "find":
		isFind = true
	}
	session, _ := store.Get(r, APP)
	if f.IsDir() {
		var fs []string
		var list []os.FileInfo
		if isRead {
			fs, err = ioReadDir(path)
		}
		if isFind {
			fs, err = filePathWalkDir(path)
		}
		if err != nil {
			NewErrResp(w, 1, err)
			return
		}
		list, err = slice2fileinfo(fs)
		if err != nil {
			NewErrResp(w, 1, err)
			return
		}
		dir := NewDir()
		dir.Dir = m["path"]
		dir.Hash = hash(path)
		dir.UpDir = upDir(dir.Dir)

		meta := kfs.NewMeta(path)
		for _, f := range list {
			nf := NewFile(f.Name())
			nf.Hash = hash(filepath.Join(path, f.Name()))
			if isRead {
				fullPath := filepath.Join(path, f.Name())
				nf.Size, err = dirSize(fullPath)
				if err != nil {
					log.Println(err)
				}
			}
			if isFind {
				nf.Size = f.Size()
			}
			nf.SizeH = humanize.IBytes(uint64(nf.Size))
			nf.ModTime = f.ModTime()
			nf.ModTimeH = prettyTime(nf.ModTime)
			nf.IsDir = f.IsDir()
			if nf.IsDir {
				nf.Name += "/"
			}
			if m, ok := meta.Get(nf.Name); ok {
				nf.Meta = m
			}
			dir.Files = append(dir.Files, nf)
		}
		desc := true
		if des, ok := session.Values["desc"]; ok {
			d := des.([]string)
			switch d[0] {
			case "0":
				desc = false
			case "1":
				desc = true
			default:
				log.Println(d)
			}
		}
		if sortby, ok := session.Values["sortby"]; ok {
			s := sortby.([]string)
			switch s[0] {
			case "name":
				sort.Slice(dir.Files, func(i, j int) bool {
					b := dir.Files[i].Name < dir.Files[j].Name
					if desc {
						return !b
					}
					return b
				})
			case "modtime":
				sort.Slice(dir.Files, func(i, j int) bool {
					b := dir.Files[i].ModTime.Before(dir.Files[j].ModTime)
					if desc {
						return !b
					}
					return b
				})
			case "size":
				sort.Slice(dir.Files, func(i, j int) bool {
					b := dir.Files[i].Size < dir.Files[j].Size
					if desc {
						return !b
					}
					return b
				})
			}
		} else {
			sort.Slice(dir.Files, func(i, j int) bool {
				b := dir.Files[i].ModTime.After(dir.Files[j].ModTime)
				if desc {
					return !b
				}
				return b
			})
		}
		NewResp(w, dir)
	}
}

func filePathWalkDir(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func ioReadDir(root string) ([]string, error) {
	var files []string
	fileInfo, err := ioutil.ReadDir(root)
	if err != nil {
		return files, err
	}

	for _, file := range fileInfo {
		files = append(files, filepath.Join(root, file.Name()))
	}
	return files, nil
}

func slice2fileinfo(s []string) ([]os.FileInfo, error) {
	var fs []os.FileInfo
	for _, v := range s {
		if needHide(v) {
			continue
		}
		f, err := os.Stat(v)
		if err != nil {
			return fs, err
		}
		fs = append(fs, f)
	}
	return fs, nil
}
