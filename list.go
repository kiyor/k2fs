package main

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	retryablehttp "github.com/hashicorp/go-retryablehttp"
	"github.com/kiyor/golib"
	"github.com/kiyor/k2fs/lib"
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
	b, err := io.ReadAll(r.Body)
	if err != nil {
		log.Println(err)
		NewErrResp(w, 1, err)
		return
	}
	// log.Println(string(b))
	err = json.Unmarshal(b, &m)
	if err != nil {
		log.Println(string(b), err)
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
	session, err := store.Get(r, APP)
	if err != nil {
		log.Println(err)
	}
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
			host := "http://" + r.Host
			if len(flagHost) > 0 {
				host = flagHost
			}
			if isVideo(nf.Name) {
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
					nf.ShortCut = host + replacer.Replace(fp)
				}
			} else {
				nf.ShortCut = host + replacer.Replace(fp)
			}
			dir.Files = append(dir.Files, nf)
		}
		desc := true
		if m["desc"] != "" {
			session.Values["desc"] = []string{m["desc"]}
		}
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
		if m["sortby"] != "" {
			session.Values["sortby"] = []string{m["sortby"]}
		}
		if sortby, ok := session.Values["sortby"]; ok {
			s := sortby.([]string)
			// log.Println("sortby", s[0], "desc", desc)
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
		client := retryablehttp.NewClient()
		client.HTTPClient.Timeout = 2 * time.Second
		// 		client := &http.Client{
		// 			Timeout: 2 * time.Second,
		// 		}

		var tasks []golib.Task
		var cdn bool

		// 		if !isPhone(r) {
		cdn = true
		// 		}

		for _, _v := range dir.Files {
			v := _v
			fc := func() error {
				name := strings.TrimRight(v.Name, "/")
				name = filepath.Base(name)
				found := false
				if name, b := isAV(name); b {
					key := "AV:" + name
					if cdn {
						key += ":cdn"
					}
					var jr JavResp
					if b := lib.Redis.GetValue(key, &jr); b {
						if jr.Data.UserData.Like {
							v.Description += `♥️`
						}
						if jr.Data.UserData.Score == 5 {
							v.Description += `🔥`
						}
						for i := 0; i < jr.Data.UserData.FavourCount; i++ {
							v.Description += `👍`
						}
						v.Description += jr.Data.Title
						v.ThumbLink = jr.Data.BackupCover
						// tags
						m := make(map[string]bool)
						for _, t := range jr.Data.Tags {
							m[t] = true
						}
						for _, g := range jr.Data.Genre {
							m[g.Name] = true
						}
						if len(jr.Data.Fc2Uploader.Name) > 0 {
							m[jr.Data.Fc2Uploader.Name] = true
						}
						for _, s := range jr.Data.Star {
							m[s.Name] = true
						}
						var tags []string
						for k := range m {
							tags = append(tags, k)
						}
						v.Tags = sort.StringSlice(tags)

						if jr.Data.ID > 0 {
							found = true
						}
					} else {
						link := fmt.Sprintf("http://%s/v1/api?action=get_movie&name=%s", metaHost, name)
						if cdn {
							link += "&cdn=1"
						} else {
							link += "&cdn=0"
						}

						req, err := retryablehttp.NewRequest("GET", link, nil)
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
						log.Println(toJSON(jr))
						ttl := 36000 // if not found, cache for 10 hours
						if jr.Data.ID > 0 {
							ttl = 2592000 // if found, cache for 30 days
						}
						lib.Redis.SetValueWithTTL(key, jr, ttl)
						if jr.Data.UserData.Like {
							v.Description += `♥️`
						}
						if jr.Data.UserData.Score == 5 {
							v.Description += `🔥`
						}
						for i := 0; i < jr.Data.UserData.FavourCount; i++ {
							v.Description += `👍`
						}
						v.Description += jr.Data.Title
						v.ThumbLink = jr.Data.BackupCover
						// tags
						m := make(map[string]bool)
						for _, t := range jr.Data.Tags {
							m[t] = true
						}
						for _, g := range jr.Data.Genre {
							m[g.Name] = true
						}
						if len(jr.Data.Fc2Uploader.Name) > 0 {
							m[jr.Data.Fc2Uploader.Name] = true
						}
						for _, s := range jr.Data.Star {
							m[s.Name] = true
						}
						if len(jr.Data.Studio.Name) > 0 {
							m[jr.Data.Studio.Name] = true
						}
						if len(jr.Data.Label.Name) > 0 {
							m[jr.Data.Label.Name] = true
						}
						if len(jr.Data.Series.Name) > 0 {
							m[jr.Data.Series.Name] = true
						}
						if len(jr.Data.Director.Name) > 0 {
							m[jr.Data.Director.Name] = true
						}
						var tags []string
						for k := range m {
							tags = append(tags, k)
						}
						v.Tags = sort.StringSlice(tags)
						//
						log.Println(name, "MISS")
						if jr.Data.ID > 0 {
							found = true
						}
					}
					// 					log.Println(name, jr.Data.ID)
				}
				if name, b := isSearchable(name); !found && b {
					// 					log.Println("SEARCH", name)
					res, err := lib.NewSearchClient().Search(name)
					if err == nil {
						v.Description = `❗` + res.Title
					} else {
						log.Println(err)
					}
				}
				return nil
			}
			tasks = append(tasks, golib.NewTask(fc, nil, false))
		}
		golib.NewManager(runtime.NumCPU()*10, 10000).Do(tasks)
		if isSearch {
			match := func(name string) bool {
				if strings.HasPrefix(filter, "!") {
					return !strings.Contains(name, filter[1:])
				} else {
					return strings.Contains(name, filter)
				}
			}
			var files []*File
			for _, nf := range dir.Files {
				if nf.IsDir {
					if match(nf.Name) {
						files = append(files, nf)
					} else {
						for _, v := range nf.Tags {
							if match(v) {
								files = append(files, nf)
								break
							}
						}
					}
				}
			}
			sort.Slice(files, func(i, j int) bool {
				return files[i].ModTime.After(files[j].ModTime)
			})
			dir.Files = files
		}
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
	ID          int       `json:"Id"`
	Name        string    `json:"Name"`
	Key         string    `json:"Key"`
	Title       string    `json:"Title"`
	BackupCover string    `json:"BackupCover"`
	CreatedAt   int       `json:"CreatedAt"`
	ReleaseDate time.Time `json:"ReleaseDate"`
	Length      int       `json:"Length"`
	DirectorID  int       `json:"DirectorId"`
	Director    struct {
		Name string `json:"Name"`
	} `json:"Director"`
	StudioID int `json:"StudioId"`
	Studio   struct {
		Name string `json:"Name"`
	} `json:"Studio"`
	LabelID int `json:"LabelId"`
	Label   struct {
		Name string `json:"Name"`
	} `json:"Label"`
	SeriesID int `json:"SeriesId"`
	Series   struct {
		Name string `json:"Name"`
	} `json:"Series"`
	Fc2UploaderID int `json:"Fc2UploaderId"`
	Fc2Uploader   struct {
		Name string `json:"Name"`
	}
	Star []struct {
		Name string `json:"Name"`
	}
	Tags       []string `json:"Tags"`
	Genre      []*Genre `json:"Genre"`
	Uncensored bool     `json:"Uncensored"`
	UserData   struct {
		Like        bool `json:"Like"`
		FavourCount int  `json:"FavourCount"`
		Score       int8 `json:"Score"`
	} `json:"UserData"`
}

type Genre struct {
	Name string
}

var (
	reAV = []*regexp.Regexp{
		regexp.MustCompile(`^[A-Z]+\-\d+$`),
		regexp.MustCompile(`^\d{3}[A-Z]+\-\d+$`),
		regexp.MustCompile(`^KIN8\-\d+$`),
		regexp.MustCompile(`^T28\-\d+$`),
		regexp.MustCompile(`^\d+\-\d+\-CARIB$`),
	}
	reSearchable = []*regexp.Regexp{
		regexp.MustCompile(`^[a-zA-Z]{2,4}\-\d{2,4}$`),
		regexp.MustCompile(`^zb\d{8}_\d+$`),
	}
	mapAV      = make(map[string]string)
	idreplacer = strings.NewReplacer(
		"-C_X1080X", "",
		"-C_GG5", "",
		"[MD]", "",
	)
)

func isSearchable(name string) (string, bool) {
	if strings.HasPrefix(name, "FC2-PPV") {
		return strings.Split(name, ".")[0], true
	}
	for _, re := range reSearchable {
		if re.MatchString(name) {
			return name, true
		}
	}
	return name, false
}

func isAV(name string) (string, bool) {
	name = idreplacer.Replace(name)
	if strings.Contains(strings.ToLower(name), "gitchu") {
		re := regexp.MustCompile(`(gitchu\-\d+)`)
		if re.MatchString(name) {
			name = re.ReplaceAllString(name, "$1")
			return name, true
		}
	}
	if strings.HasPrefix(name, "FC2-PPV") {
		return strings.Split(name, ".")[0], true
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
	fileInfo, err := os.ReadDir(root)
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
