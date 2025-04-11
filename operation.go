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

	"github.com/kiyor/k2fs/lib"
	kfs "github.com/kiyor/k2fs/lib"
)

// Operation api request
type Operation struct {
	Files  map[string]bool `json:"files"`
	Dir    string          `json:"dir"`
	Action string          `json:"action"`
}

func (o *Operation) ActionKey() string {
	return strings.Split(o.Action, "=")[0]
}

func (o *Operation) ActionValue() string {
	s := strings.Split(o.Action, "=")
	if len(s) > 1 {
		return s[1]
	}
	return ""
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
	for k, b := range op.Files {
		file := filepath.Join(path, k)
		if b {
			m, _ := meta.Get(k)
			key := filepath.Join(op.Dir, k)
			// log.Println("-------------->", key)
			m2, err := metaV2.Get(key)
			if err != nil {
				m2, err = metaV2.LoadPath(key)
				if err != nil {
					// log.Println(key, err)
					continue
				}
			}
			// log.Println("-------------->", toJSON(m2))
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
				m2.SetLabel(m.Label)
			case strings.HasPrefix(op.Action, "mark"):
				switch op.ActionValue() {
				case "5":
					m2.SetLabel("danger")
					m2.SetStar(true)
					m.Label = "danger"
					m.Star = true
					meta.Set(k, m)
				case "4":
					m2.SetLabel("danger")
					m.Label = "danger"
					meta.Set(k, m)
				default:
				}
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
				m2.SetStar(m.Star)
			case strings.HasPrefix(op.Action, "star"):
				to := strings.Split(op.Action, "=")
				if len(to) > 1 && len(to[1]) > 0 {
					m.Star = true
				} else {
					m.Star = false
				}
				meta.Set(k, m)
				m2.SetStar(m.Star)
			case op.Action == "restore":
				if strings.HasPrefix(file, Trash) {
					dst := filepath.Join(m.OldLoc)
					log.Println("mv", file, dst)
					os.Rename(file, dst)
					dstMeta := kfs.NewMeta(filepath.Dir(dst))
					dstMeta.Set(k, m)
					dstMeta.Write()
					meta.Del(k)
					lib.Cache.Remove("size:.Trash")
				}
			case op.Action == "delete":
				log.Println(file, Trash)
				trashMeta := kfs.NewMeta(Trash)
				// delete trash, delete all file in trash
				if file == Trash {
					fs, _ := os.ReadDir(Trash)
					for _, v := range fs {
						f := filepath.Join(Trash, v.Name())
						err := os.RemoveAll(f)
						log.Println("rm -rf", f, err)
					}
				} else if strings.HasPrefix(file, Trash) { // file inside trash, delete single file
					err := os.RemoveAll(file)
					log.Println("rm -rf", file, err)
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
					metaV2.MoveDir(key, ".Trash")
				}
				lib.Cache.Remove("size:.Trash")
				metaV2.RemoveOrphan(".Trash")
				metaV2.Index(".Trash")
				dirSize2(".Trash")
			}
		}
	}
	err = meta.Write()
	if err != nil {
		log.Println(err)
		NewResp(w, err, nil, 1)
		return
	}
	NewResp(w, "success", nil)
}
