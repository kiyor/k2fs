package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
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
	"time"

	"github.com/dustin/go-humanize"
	"github.com/kiyor/golib"
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
	reMac1 = regexp.MustCompile(`Macintosh; Intel Mac OS X .*\) AppleWebKit\/.* \(KHTML, like Gecko\) Chrome\/.* Safari\/.*`)
	reMac2 = regexp.MustCompile(`Macintosh; Intel Mac OS X .*\) Gecko\/.* Firefox\/.*`)
	reWin  = regexp.MustCompile(`Mozilla\/.* \(Windows NT .*; Win.*; .*\) AppleWebKit\/.* \(KHTML, like Gecko\) Chrome\/.* Safari\/.*`)
)

func isMac(r *http.Request) bool {
	ag := r.Header.Get("User-Agent")
	if reMac1.MatchString(ag) {
		return true
	}
	if reMac2.MatchString(ag) {
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
func isPhone(r *http.Request) bool {
	ag := r.Header.Get("User-Agent")
	if rePhone.MatchString(ag) {
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
	d, _ := filepath.Split(path)
	return strings.TrimRight(d, "/")
}

type Thumb struct {
	Path   string
	Width  int
	Height int
}

func buildCacheKey(r *http.Request, i interface{}) string {
	return r.URL.Path + toJSON(r.URL.Query()) + toJSON(i)
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
	cacheKey := buildCacheKey(r, m)
	if val, err := cache.Get(cacheKey); err == nil {
		w.Header().Add("content-type", "application/json")
		w.Write(val.([]byte))
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
	abs := filepath.Join(rootDir, path)

	f, err := os.Stat(abs)
	if err != nil {
		log.Println(err)
		NewErrResp(w, 1, err)
		return
	}
	pathEscape := func(input string) string {
		return strings.ReplaceAll(url.PathEscape(input), "%2F", "/")
	}
	fp := func(p string) *Thumb {
		path := filepath.Join("/statics", p)
		if reader, err := os.Open(filepath.Join(rootDir, p)); err == nil {
			defer reader.Close()
			im, _, err := image.DecodeConfig(reader)
			if err != nil {
				log.Println(err)
				return &Thumb{
					Path: pathEscape(path),
				}
			}
			width, height := im.Width, im.Height
			if width > 1200 {
				width = 1200
				height = int(float64(width) / float64(im.Width) * float64(im.Height))
			}
			return &Thumb{
				Path:   pathEscape(path),
				Width:  width,
				Height: height,
			}
		} else {
			log.Println(err)
			return &Thumb{
				Path: pathEscape(filepath.Join("/statics", p)),
			}
		}
	}
	if f.IsDir() {
		fs := readDir2(abs)
		if len(fs) == 0 {
			NewCacheResp(w, "", cacheKey, time.Hour)
			// 			log.Println("MISS", cacheKey)
			return
		}
		for _, v := range fs {
			if strings.HasSuffix(strings.ToLower(v), "cover.") {
				NewCacheResp(w, fp(v), cacheKey, time.Hour)
				// 				log.Println("MISS", cacheKey)
				return
			}
		}
		sort.Strings(fs)
		NewCacheResp(w, fp(fs[0]), cacheKey, time.Hour)
		// 		log.Println("MISS", cacheKey)
		return
	}
}

var imageExt = []string{".JPG", ".JPEG", ".PNG", ".GIF", ".BMP"}

func isImage(path string) bool {
	ext := strings.ToUpper(filepath.Ext(path))
	for _, v := range imageExt {
		if ext == v {
			return true
		}
	}
	return false
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
	filter := m["search"]
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
	var isRead, isFind, isSearch bool
	switch m["listdir"] {
	case "read":
		isRead = true
	case "find":
		isFind = true
	}
	if len(filter) > 0 {
		isFind = true
		isRead = false
		isSearch = true
	}
	session, _ := store.Get(r, APP)
	if f.IsDir() {
		var fs []string
		var list map[string]os.FileInfo
		if isRead {
			fs, err = ioReadDir(abs)
		}
		if isFind {
			fs, err = filePathWalkDir(abs, isSearch)
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
		if isRead {
			for _, f := range list {
				dirSize(filepath.Join(abs, f.Name()))
			}
		}
		time.Sleep(200 * time.Millisecond)

		//TODO optimize search/filter, do before some action like size()
		for p, f := range list {
			nf := NewFile(f.Name())
			nf.Hash = hash(filepath.Join(abs, f.Name()))
			if isRead {
				fullPath := filepath.Join(abs, f.Name())
				nf.Size, err = dirSize(fullPath)
				if err != nil {
					log.Println(err)
				}
			}
			if isFind {
				nf.Size = f.Size()
			}
			nf.Path = p
			nf.SizeH = humanize.IBytes(uint64(nf.Size))
			nf.ModTime = f.ModTime()
			nf.ModTimeH = prettyTime(nf.ModTime)
			nf.IsDir = f.IsDir()
			if nf.IsDir {
				nf.Name += "/"
			} else {
				nf.IsImage = isImage(nf.Path)
			}
			d, _ := filepath.Split(nf.Path)
			meta := kfs.NewMeta(filepath.Join(rootDir, d))
			if m, ok := meta.Get(nf.Name); ok {
				nf.Meta = m
			}
			// 			fp := filepath.Join("/statics", path, f.Name())
			fp := filepath.Join("/statics", p)
			replacer := strings.NewReplacer("+", "%20", "#", "%23")
			if isVideo(nf.Name) {
				host := "http://" + r.Host
				if len(flagHost) > 0 {
					host = flagHost
				}
				qv := url.Values{}
				qv["url"] = []string{host + fp}
				q := replacer.Replace(qv.Encode())
				switch {
				case isMac(r):
					nf.ShortCut = "iina://open?" + q
					// 				case isPhone(r):
					// 					nf.ShortCut = "iina://open?" + q
				case isWin(r):
					nf.ShortCut = host + replacer.Replace(fp)
				default:
					nf.ShortCut = replacer.Replace(fp)
				}
			} else {
				nf.ShortCut = replacer.Replace(fp)
			}
			/*
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
						host = flagHost
					}
					replacer := strings.NewReplacer("#", "%23")
					nf.ShortCut = host + replacer.Replace(fp)
					log.Println(nf.ShortCut)
				} else if isPhone(r) && isVideo(nf.Name) {
					host := "http://" + r.Host
					if len(flagHost) > 0 {
						host = flagHost
					}
					qv := url.Values{}
					qv["url"] = []string{host + fp}
					replacer := strings.NewReplacer("+", "%20") //, "#", "%23")
					q := replacer.Replace(qv.Encode())
					nf.ShortCut = "vlc-x-callback://x-callback-url/stream?" + q
					log.Println(nf.ShortCut)
				} else {
					replacer := strings.NewReplacer("#", "%23")
					nf.ShortCut = replacer.Replace(fp)
				}
			*/
			if isSearch {
				if nf.IsDir {
					if strings.Contains(nf.Name, filter) {
						dir.Files = append(dir.Files, nf)
					} else {
						for _, v := range nf.Meta.Tags {
							if strings.Contains(v, filter) {
								dir.Files = append(dir.Files, nf)
								break
							}
						}
					}
				}
			} else {
				dir.Files = append(dir.Files, nf)
			}
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
		// query thumb start
		client := &http.Client{
			Timeout: 2 * time.Second,
		}

		var tasks []golib.Task

		for _, _v := range dir.Files {
			v := _v
			fc := func() error {
				name := strings.TrimRight(v.Name, "/")
				name = filepath.Base(name)
				if name, b := isAV(name); b {
					key := "AV:" + name
					var jr JavResp
					if b := Redis.GetValue(key, &jr); b {
						v.Description = jr.Data.Title
						v.ThumbLink = jr.Data.BackupCover
						v.Tags = jr.Data.Tags
					} else {
						req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/v1/api?action=get_movie&name=%s", metaHost, name), nil)
						if err != nil {
							log.Println(err)
							return err
						}
						resp, err := client.Do(req)
						if err != nil {
							log.Println(err)
							return err
						}
						defer resp.Body.Close()
						var jr JavResp
						err = json.NewDecoder(resp.Body).Decode(&jr)
						if err != nil {
							log.Println(err)
							return err
						}
						ttl := 600
						if jr.Data.ID > 0 {
							ttl = 7 * 86400
						}
						Redis.SetValueWithTTL(key, jr, ttl)
						v.Description = jr.Data.Title
						v.ThumbLink = jr.Data.BackupCover
						v.Tags = jr.Data.Tags
						log.Println(name, "MISS")
					}
				}
				return nil
			}
			tasks = append(tasks, golib.NewTask(fc, nil, false))
		}
		golib.NewManager(10, 10000).Do(tasks)
		// query thumb end
		NewResp(w, dir)
	}
}

func init() {
	gob.Register(new(JavResp))
}

type JavResp struct {
	Code int     `json:"Code"`
	Data JavData `json:"Data"`
}
type JavData struct {
	ID            int       `json:"Id"`
	Name          string    `json:"Name"`
	Key           string    `json:"Key"`
	Title         string    `json:"Title"`
	BackupCover   string    `json:"BackupCover"`
	CreatedAt     int       `json:"CreatedAt"`
	ReleaseDate   time.Time `json:"ReleaseDate"`
	Length        int       `json:"Length"`
	DirectorID    int       `json:"DirectorId"`
	StudioID      int       `json:"StudioId"`
	LabelID       int       `json:"LabelId"`
	SeriesID      int       `json:"SeriesId"`
	Fc2UploaderID int       `json:"Fc2UploaderId"`
	Tags          []string  `json:"Tags"`
	Uncensored    bool      `json:"Uncensored"`
}

var (
	reAV = []*regexp.Regexp{
		regexp.MustCompile(`^[A-Z]+\-\d+$`),
		regexp.MustCompile(`^\d{3}[A-Z]+\-\d+$`),
		regexp.MustCompile(`^KIN8\-\d+$`),
		regexp.MustCompile(`^T28\-\d+$`),
	}
	mapAV      = make(map[string]string)
	idreplacer = strings.NewReplacer("-C_X1080X", "", "-C_GG5", "")
)

func isAV(name string) (string, bool) {
	name = idreplacer.Replace(name)
	if strings.HasPrefix(name, "FC2-PPV") {
		return name, true
	}
	reIBW := regexp.MustCompile(`^(IBW\-\d+)Z$`)
	if reIBW.MatchString(name) {
		name = reIBW.ReplaceAllString(name, "$1")
		return name, true
	}
	for _, re := range reAV {
		if re.MatchString(name) {
			return name, true
		}
	}
	return name, false
}

func filePathWalkDir(root string, isDir bool) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() == isDir {
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
	l := len(filepath.Join(rootDir)) //, prefix))
	for _, v := range s {
		if needHide(v) {
			continue
		}
		f, err := os.Stat(v)
		if err != nil {
			return fs, err
		}
		// 		log.Println(v, prefix)
		if len(v) > l {
			fs[v[l+1:]] = f
			// 			log.Println(v[l+1:])
		}
	}
	return fs, nil
}
