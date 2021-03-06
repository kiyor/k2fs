package main

import (
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	kfs "github.com/kiyor/k2fs/lib"
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
	".HTML",
	kfs.KFS,
}
var hideContain = []string{
	"padding_file",
}
var videoExt = []string{
	".mp4",
	".mov",
	".avi",
	".wmv",
	".mkv",
	".ts",
	".flv",
	".mpg",
	".dat",
}

func isVideo(file string) bool {
	ext := strings.ToLower(filepath.Ext(file))
	for _, v := range videoExt {
		if v == ext {
			return true
		}
	}
	return false
}

var (
	reMac = regexp.MustCompile(`Macintosh; Intel Mac OS X .*\) AppleWebKit\/.* \(KHTML, like Gecko\) Chrome\/.* Safari\/.*`)
	reWin = regexp.MustCompile(`Mozilla\/.* \(Windows NT .*; Win.*; .*\) AppleWebKit\/.* \(KHTML, like Gecko\) Chrome\/.* Safari\/.*`)
)

func isMac(r *http.Request) bool {
	ag := r.Header.Get("User-Agent")
	if reMac.MatchString(ag) {
		return true
	}
	return false
}
func isWin(r *http.Request) bool {
	ag := r.Header.Get("User-Agent")
	if reWin.MatchString(ag) {
		return true
	}
	return false
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

type Thumb struct {
	Path   string
	Width  int
	Height int
}

func apiThumb(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	m := make(map[string]string)
	err := json.NewDecoder(r.Body).Decode(&m)
	if err != nil {
		log.Println(err)
		NewErrResp(w, 1, err)
		return
	}
	path := m["path"]
	if strings.Contains(path, "%") {
		path, _ = url.PathUnescape(path)
	}
	path = strings.TrimRight(path, "/")
	if len(path) == 0 {
		path = "/"
	}
	m["name"] = strings.TrimRight(m["name"], "/")
	abs := filepath.Join(rootDir, path, m["name"])

	f, err := os.Stat(abs)
	if err != nil {
		log.Println(err)
		NewErrResp(w, 1, err)
		return
	}
	fp := func(p string) *Thumb {
		path := filepath.Join("/statics", p)
		if reader, err := os.Open(filepath.Join(rootDir, p)); err == nil {
			defer reader.Close()
			im, _, err := image.DecodeConfig(reader)
			if err != nil {
				log.Println(err)
				return &Thumb{
					Path: path,
				}
			}
			width, height := im.Width, im.Height
			if width > 1200 {
				width = 1200
				height = int(float64(width) / float64(im.Width) * float64(im.Height))
			}
			return &Thumb{
				Path:   path,
				Width:  width,
				Height: height,
			}
		} else {
			log.Println(err)
			return &Thumb{
				Path: filepath.Join("/statics", p),
			}
		}
	}
	if f.IsDir() {
		fs := readDir2(abs)
		if len(fs) == 0 {
			NewResp(w, "")
			return
		}
		for _, v := range fs {
			if strings.HasSuffix(strings.ToLower(v), "cover.jpg") {
				NewResp(w, fp(v))
				return
			}
		}
		sort.Strings(fs)
		NewResp(w, fp(fs[0]))
		return
	}
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
	path := m["path"]
	if strings.Contains(path, "%") {
		path, _ = url.PathUnescape(path)
	}
	path = strings.TrimRight(path, "/")
	if len(path) == 0 {
		path = "/"
	}
	if _, ok := m["listdir"]; !ok {
		m["listdir"] = "read"
	}
	abs := filepath.Join(rootDir, path)
	f, err := os.Stat(abs)
	if err != nil {
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
		var list map[string]os.FileInfo
		if isRead {
			fs, err = ioReadDir(path)
		}
		if isFind {
			fs, err = filePathWalkDir(path)
		}
		if err != nil {
			log.Println(err)
			NewErrResp(w, 1, err)
			return
		}
		list, err = slice2fileinfo(fs, path)
		if err != nil {
			log.Println(err)
			NewErrResp(w, 1, err)
			return
		}
		dir := NewDir()
		dir.Dir = path
		dir.Hash = hash(path)
		dir.UpDir = upDir(dir.Dir)

		meta := kfs.NewMeta(path)
		for p, f := range list {
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
				nf.Path = p
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
			fp := filepath.Join("/statics", path, p)
			if isMac(r) && isVideo(nf.Name) {
				host := "http://" + r.Host
				if len(flagHost) > 0 {
					host = flagHost
				}
				qv := url.Values{}
				qv["url"] = []string{host + fp}
				replacer := strings.NewReplacer("+", "%20", "#", "%23")
				q := replacer.Replace(qv.Encode())
				nf.ShortCut = "iina://open?" + q
			} else if isWin(r) && isVideo(nf.Name) {
				// 				host := "vlc://" + r.Host
				host := "http://" + r.Host
				if len(flagHost) > 0 {
					// 					rp := strings.NewReplacer("http", "vlc", "https", "vlc")
					// 					host = rp.Replace(flagHost)
					host = flagHost
				}
				replacer := strings.NewReplacer("#", "%23")
				nf.ShortCut = host + replacer.Replace(fp)
				log.Println(nf.ShortCut)
			} else {
				replacer := strings.NewReplacer("#", "%23")
				nf.ShortCut = replacer.Replace(fp)
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

func slice2fileinfo(s []string, prefix string) (map[string]os.FileInfo, error) {
	fs := make(map[string]os.FileInfo)
	for _, v := range s {
		if needHide(v) {
			continue
		}
		f, err := os.Stat(v)
		if err != nil {
			return fs, err
		}
		fs[v[len(prefix)+1:]] = f
	}
	return fs, nil
}
