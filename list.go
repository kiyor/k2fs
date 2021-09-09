package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/dustin/go-humanize"
	kfs "github.com/kiyor/kfs/lib"
)

func apiList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// 	path := "." + q.Get("path")
	path := filepath.Join(rootDir, q.Get("path"))
	f, err := os.Stat(path)
	if err != nil {
		w.Write(NewErrResp(1, err))
		return
	}
	session, _ := store.Get(r, APP)
	if f.IsDir() {
		fs, err := ioutil.ReadDir(path)
		if err != nil {
			w.Write(NewErrResp(1, err))
			return
		}
		dir := NewDir()
		meta := kfs.NewMeta(path)
		for _, f := range fs {
			if f.Name() == kfs.KFS {
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
		w.Write(NewResp(dir))
	}
}
