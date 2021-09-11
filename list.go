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
	kfs.KFS,
}

func needHide(path string) bool {
	for _, v := range hideExt {
		if strings.ToUpper(filepath.Ext(path)) == v {
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
	path := filepath.Join(rootDir, m["path"])
	f, err := os.Stat(path)
	if err != nil {
		log.Println(err)
		NewErrResp(w, 1, err)
		return
	}
	session, _ := store.Get(r, APP)
	if f.IsDir() {
		fs, err := ioutil.ReadDir(path)
		if err != nil {
			NewErrResp(w, 1, err)
			return
		}
		dir := NewDir()
		dir.Dir = m["path"]
		dir.Hash = hash(path)
		dir.UpDir = upDir(dir.Dir)

		meta := kfs.NewMeta(path)
		for _, f := range fs {
			if needHide(f.Name()) {
				continue
			}
			nf := NewFile(f.Name())
			nf.Hash = hash(filepath.Join(path, f.Name()))
			fullPath := filepath.Join(path, f.Name())
			nf.Size, err = dirSize(fullPath)
			if err != nil {
				log.Println(err)
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
