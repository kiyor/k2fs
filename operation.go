package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	kfs "github.com/kiyor/k2fs/lib"
)

// Operation api request
type Operation struct {
	Files  map[string]bool `json:"files"`
	Dir    string          `json:"dir"`
	Action string          `json:"action"`
}

var Trash string
var opMutex *sync.Mutex = new(sync.Mutex)

func apiOperation(w http.ResponseWriter, r *http.Request) {
	opMutex.Lock()
	defer opMutex.Unlock()
	defer r.Body.Close()
	var op Operation
	err := json.NewDecoder(r.Body).Decode(&op)
	if err != nil {
		NewErrResp(w, 1, err)
		return
	}

	// log.Println(toJSON(op))

	path := filepath.Join(rootDir, op.Dir)

	meta := kfs.NewMeta(path)
	// 	log.Println(toJSON(meta))
	for k, b := range op.Files {
		file := filepath.Join(path, k)
		if b {
			m, _ := meta.Get(k)
			switch {
			case op.Action == "unzip":
				var cmd string
				var c *exec.Cmd
				d := filepath.Dir(file)
				f := filepath.Base(file)
				b := filepath.Join(d, f[:len(f)-4])
				err := os.Mkdir(b, 0755)
				if err != nil {
					log.Println(err)
					return
				}
				switch filepath.Ext(strings.ToLower(file)) {
				case ".rar":
					cmd = fmt.Sprintf("unrar -y e '%s' '%s'", f, b)
					c = exec.Command("/bin/sh", "-c", cmd)
				case ".zip":
					cmd = fmt.Sprintf("unzip '%s' -d '%s'", f, b)
					c = exec.Command("/bin/sh", "-c", cmd)
				}
				c.Dir = d
				log.Println(d, cmd)
				c.Stderr = os.Stderr
				c.Stdout = os.Stdout
				c.Run()
			case strings.HasPrefix(op.Action, "label"):
				to := strings.Split(op.Action, "=")
				if len(to) > 1 {
					m.Label = to[1]
				} else {
					m.Label = ""
				}
				meta.Set(k, m)
			case strings.HasPrefix(op.Action, "icons"):
				to := strings.Split(op.Action, "=")
				m.Icons = []string{}
				if len(to) > 1 {
					m.Icons = append(m.Icons, to[1])
				} else {
					m.Icons = []string{}
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
					fs, _ := os.ReadDir(Trash)
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
					err := trashMeta.Write()
					if err != nil {
						log.Println(err)
					}
				}
				cache.Remove("size:" + Trash)
			}
		}
	}
	err = meta.Write()
	if err != nil {
		log.Println(err)
		NewResp(w, err, 1)
		return
	}
	NewResp(w, "success")
	// 	log.Println(toJSON(meta))
}
