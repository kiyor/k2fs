package lib

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bluele/gcache"
	_ "github.com/mattn/go-sqlite3"
	"xorm.io/xorm"
)

var Cache = gcache.New(10000).LRU().Build()

func (m *MetaV2) db() *xorm.Engine {
	// m.mu.Lock()
	// defer m.mu.Unlock()
	if m.Engine == nil {
		path := filepath.Join(m.Root, ".kfs.db?cache=shared&_mutex=full")
		// var needIndex bool
		// if _, err := os.Stat(path); err != nil {
		// 	needIndex = true
		// }

		x, err := xorm.NewEngine("sqlite3", path)
		if err != nil {
			log.Fatal(err)
		}
		showSQL, _ := strconv.ParseBool(os.Getenv("SHOW_SQL"))
		x.ShowSQL(showSQL)
		// x.Logger().SetLevel(xlog.LOG_WARNING)

		m.Engine = x
		for _, v := range []string{
			"PRAGMA journal_mode=WAL;",
			"PRAGMA synchronous=NORMAL;",
			"PRAGMA cache_size = 10000;",
			"PRAGMA temp_store = MEMORY;",
			"PRAGMA locking_mode = EXCLUSIVE;",
			"PRAGMA busy_timeout = 30000;",
			"PRAGMA secure_delete = ON;",
		} {
			_, err := m.Engine.Exec(v)
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	return m.Engine
}

func (m *MetaV2) init() error {
	tables := []interface{}{MetaInfoV2{}}
	var errs error
	for _, v := range tables {
		err := m.db().Sync2(v)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

type MetaV2 struct {
	*xorm.Engine
	Root string
	// mu   *sync.Mutex
}

func NewMetaV2(root string) *MetaV2 {
	m := MetaV2{
		Root: root,
		// mu:   &sync.Mutex{},
	}
	err := m.init()
	if err != nil {
		log.Println(err)
	}
	return &m
}

func (m *MetaV2) LoadPath(relPath string) (*MetaInfoV2, error) {
	info, err := os.Stat(filepath.Join(m.Root, relPath))
	if err != nil {
		return nil, err
	}
	return m.NewInfo(relPath, info)
}

// MoveDir("Downloads/xxx", ".Trash") -> move all files in Downloads/xxx to .Trash/xxx
func (m *MetaV2) MoveDir(srcDir, dstDir string) error {
	srcDir = strings.TrimLeft(srcDir, "/")

	infos, err := m.List(MetaV2ListOptions{Prefix: &srcDir})
	if err != nil {
		return err
	}
	for _, i := range infos {
		orgPath := i.Path
		dirBase := filepath.Base(i.Dir)
		dstPath := filepath.Join(dstDir, dirBase, filepath.Base(i.Path))
		if i.IsDir() {
			dstPath = filepath.Join(dstDir, dirBase)
			dstDir = dstPath
		}
		i.Path = dstPath
		i.Dir = dstDir
		i.OldLoc = orgPath
		m.Set(&i)
		m.db().Where("path = ?", orgPath).Delete(&MetaInfoV2{})
	}
	return nil
}

func (m *MetaV2) NewInfo(path string, info os.FileInfo) (*MetaInfoV2, error) {
	i, err := m.Get(path)
	if err != nil {
		// file not exist
		var dir string
		// log.Println(path, "isdir", info.IsDir())
		if info.IsDir() {
			dir = path
		} else {
			dir = filepath.Dir(path)
		}
		i := &MetaInfoV2{
			Path:    path,
			Dir:     dir,
			Size:    info.Size(),
			ModTime: info.ModTime(),
		}
		m.Set(i)
		return i, nil
	}
	if i.ModTime == info.ModTime() && i.Size == info.Size() {
		return i, nil
	} else {
		i.Size = info.Size()
		i.ModTime = info.ModTime()
		m.Set(i)
	}
	return i, nil
}

func (m *MetaV2) Get(path string) (*MetaInfoV2, error) {
	info := MetaInfoV2{
		Path:   path,
		MetaV2: m,
	}
	has, err := m.db().Get(&info)
	if err != nil {
		return nil, err
	}
	if has {
		return &info, nil
	} else {
		return nil, fmt.Errorf("not found")
	}
}

type MetaInfoV2 struct {
	Path    string    `json:"path" xorm:"pk"`
	Dir     string    `json:"dir" xorm:"index"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Label   string    `json:"label"`
	Tags    []string  `json:"tags"`
	Star    bool      `json:"star"`
	Icons   []string  `json:"icons"`
	OldLoc  string
	Context map[string]interface{}
	MetaV2  *MetaV2 `xorm:"-"`
}
type MetaInfoV2s []MetaInfoV2

func (i *MetaInfoV2) GetDir() string {
	return filepath.Dir(i.Path)
}

func (i *MetaInfoV2) IsDir() bool {
	return i.Path == i.Dir
}
func (i *MetaInfoV2) IsTrash() bool {
	return i.Dir == filepath.Join(i.Dir, ".Trash")
}

func (i *MetaInfoV2) AfterLoad() {
}

func (i *MetaInfoV2) SetLabel(label string) {
	i.Label = label
	i.MetaV2.Set(i)
}

func (i *MetaInfoV2) SetStar(star bool) {
	i.Star = star
	i.MetaV2.Set(i)
}

func (m *MetaV2) Index(prefixs ...string) error {
	if len(prefixs) == 0 {
		prefixs = append(prefixs, "")
	}
	for _, prefix := range prefixs {
		prefix = strings.TrimLeft(prefix, "/")
		err := filepath.Walk(filepath.Join(m.Root, prefix), func(path string, info os.FileInfo, err error) error {
			path, err = filepath.Rel(m.Root, path)
			if err != nil {
				return err
			}
			m.NewInfo(path, info)
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *MetaV2) RemoveOrphan(prefixs ...string) error {
	if len(prefixs) == 0 {
		prefixs = append(prefixs, "")
	}
	for _, prefix := range prefixs {
		is, err := m.List(MetaV2ListOptions{Prefix: &prefix})
		if err != nil {
			return err
		}
		for _, i := range is {
			if _, err := os.Stat(filepath.Join(m.Root, i.Path)); err != nil {
				m.Del(i.Path)
			}
		}
	}
	return nil
}

func (m *MetaV2) CacheSize() error {
	var infos MetaInfoV2s
	err := m.db().Distinct("dir").Cols("dir").Find(&infos)
	if err != nil {
		return err
	}
	for _, i := range infos {
		size, err := m.Size(i.Dir)
		if err != nil {
			return err
		}
		Cache.SetWithExpire("size:"+i.Dir, size, time.Hour)
		time.Sleep(time.Millisecond * 1)
		// log.Println("cache size", i.Dir, size)
	}

	return nil
}

func (m *MetaV2) Close() error {
	m.db().Close()
	m.Engine = nil
	return nil
}

type MetaV2ListOptions struct {
	Prefix *string
}

func (m *MetaV2) List(opts MetaV2ListOptions) (MetaInfoV2s, error) {
	var list MetaInfoV2s
	session := m.db().NewSession()
	if opts.Prefix != nil {
		session.Where("path like ?", *opts.Prefix+"%")
	}
	err := session.Find(&list)
	return list, err
}

func (m *MetaV2) Size(prefix string) (float64, error) {
	f, err := m.db().Where("path like ?", prefix+"%").Sum(&MetaInfoV2{}, "size")
	if err != nil {
		return 0, err
	}
	if f == 0.0 {
		m.Index(prefix)
		return m.db().Where("path like ?", prefix+"%").Sum(&MetaInfoV2{}, "size")
	}
	return f, nil
}

func (m *MetaV2) SizeWithTimeout(prefix string, ctx context.Context) (float64, error) {
	resultChan := make(chan float64)
	errChan := make(chan error)

	// Run Query in a goroutine so that it can be executed concurrently
	go func() {
		result, err := m.Size(prefix)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	// Use select to wait on multiple channel operations.
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return 1, err
	case <-ctx.Done():
		return 2, ctx.Err() // Return default value and the context error
	}
}

func (m *MetaV2) Set(val *MetaInfoV2) *MetaInfoV2 {
	if val == nil {
		return nil
	}
	_, err := m.Get(val.Path)
	if err != nil {
		m.db().Insert(val)
	} else {
		m.db().Where("path = ?", val.Path).Update(val)
	}
	val.MetaV2 = m
	return val
}

func (m *MetaV2) Del(path string) {
	info, err := m.Get(path)
	if err != nil {
		return
	}
	if info.IsDir() {
		m.db().Where("dir = ?", path).Delete(&MetaInfoV2{})
	} else {
		m.db().Where("path = ?", path).Delete(&MetaInfoV2{})
	}
}
