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
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.Write(NewErrResp(1, err))
		return
	}
	var op Operation
	json.Unmarshal(body, &op)
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
				if len(to) > 0 {
					m.Label = to[1]
				} else {
					m.Label = ""
				}
				meta.Set(k, m)
			case op.Action == "star":
				m.Star = !m.Star
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
			}
		}
	}
	meta.Write()
	// 	log.Println(toJSON(meta))
}
