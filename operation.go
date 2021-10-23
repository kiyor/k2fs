package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	kfs "github.com/kiyor/kfs/lib"
)

// Operation api request
type Operation struct {
	Files  map[string]bool `json:"files"`
	Dir    string          `json:"dir"`
	Action string          `json:"action"`
}

var Trash string

func apiOperation(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var op Operation
	err := json.NewDecoder(r.Body).Decode(&op)
	if err != nil {
		NewErrResp(w, 1, err)
		return
	}

	// 	log.Println(toJSON(op))

	path := filepath.Join(rootDir, op.Dir)

	meta := kfs.NewMeta(path)
	// 	log.Println(toJSON(meta))
	for k, b := range op.Files {
		file := filepath.Join(path, k)
		if b {
			m, _ := meta.Get(k)
			switch {
			case strings.HasPrefix(op.Action, "label"):
				to := strings.Split(op.Action, "=")
				if len(to) > 1 {
					m.Label = to[1]
				} else {
					m.Label = ""
				}
				meta.Set(k, m)
			case op.Action == "star":
				m.Star = !m.Star
				meta.Set(k, m)
			case strings.HasPrefix(op.Action, "star"):
				to := strings.Split(op.Action, "=")
				if len(to) > 1 && len(to[1]) > 0 {
					m.Star = true
				} else {
					m.Star = false
				}
				meta.Set(k, m)
			case op.Action == "delete":
				log.Println(file, Trash)
				trashMeta := kfs.NewMeta(Trash)
				// delete trash, delete all file in trash
				if file == Trash {
					fs, _ := ioutil.ReadDir(Trash)
					for _, v := range fs {
						f := filepath.Join(Trash, v.Name())
						log.Println("rm -rf", f)
						os.RemoveAll(f)
					}
				} else if strings.HasPrefix(file, Trash) { // file inside trash, delete single file
					log.Println("rm -rf", file)
					os.RemoveAll(file)
					meta.Del(k)
					// 					trashMeta.Write()
				} else { // not inside trash
					dst := filepath.Join(Trash, k)
					log.Println("mv", file, dst)
					os.Rename(file, dst)
					meta.Del(k)
					m.OldLoc = file
					trashMeta.Set(k, m)
					trashMeta.Write()
				}
				cache.Remove("size:" + Trash)
			}
		}
	}
	meta.Write()
	NewResp(w, "success")
	// 	log.Println(toJSON(meta))
}
