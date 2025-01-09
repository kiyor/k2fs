package lib

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bluele/gcache"
	//_ "github.com/mattn/go-sqlite3"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var Cache = gcache.New(20000).LRU().Build()

func (m *MetaV2) dbOrgPath() string {
	return filepath.Join(m.root, ".kfs.db")
}

func (m *MetaV2) dbAllOrgPath() []string {
	return []string{
		m.dbOrgPath(),
		m.dbOrgPath() + "-shm",
		m.dbOrgPath() + "-wal",
	}
}

func (m *MetaV2) dbPath() string {
	if _, err := os.Stat(m.dbDir); os.IsNotExist(err) {
		err = os.MkdirAll(m.dbDir, 0755)
		if err != nil {
			log.Fatal(err)
		}
	}
	if m.dbDir == m.root {
		return filepath.Join(m.dbDir, ".kfs.db")
	}
	return filepath.Join(m.dbDir, "kfs.db")
}

func (m *MetaV2) dbAllPath() []string {
	return []string{
		m.dbPath(),
		m.dbPath() + "-shm",
		m.dbPath() + "-wal",
	}
}

func (m *MetaV2) dbMigrate() {
	for k, v := range m.dbAllOrgPath() {
		_, err := os.Stat(v)
		if err == nil {
			p := m.dbAllPath()[k]
			f, err := os.Create(p)
			if err != nil {
				log.Fatal(err)
			}
			defer f.Close()
			o, err := os.Open(v)
			if err != nil {
				log.Fatal(err)
			}
			defer o.Close()
			_, err = io.Copy(f, o)
			if err != nil {
				log.Fatal(err)
			}
			log.Println("migrate", v, "->", p)
		}
	}
}

func (m *MetaV2) db() *gorm.DB {
	if m.DB == nil {
		if m.dbPath() != m.dbOrgPath() {
			if _, err := os.Stat(m.dbPath()); err != nil { // new db not exist
				log.Println("db not exist, try get db from org")
				if _, err := os.Stat(m.dbOrgPath()); err != nil { // old db not exist
					log.Println("db not exist, create new db")
				} else {
					m.dbMigrate()
				}
			}
		}

		path := m.dbPath() + "?cache=shared&_mutex=full"
		log.Println("db path", path)
		mod := logger.Silent
		showSQL, _ := strconv.ParseBool(os.Getenv("SHOW_SQL"))
		if showSQL {
			mod = logger.Info
		}
		db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
			Logger: logger.Default.LogMode(mod),
		})

		if err != nil {
			log.Fatal(err)
		}

		m.DB = db
		for _, v := range []string{
			"PRAGMA journal_mode=WAL;",
			"PRAGMA synchronous=NORMAL;",
			"PRAGMA cache_size = 10000;",
			"PRAGMA temp_store = MEMORY;",
			"PRAGMA locking_mode = EXCLUSIVE;",
			"PRAGMA busy_timeout = 30000;",
			"PRAGMA secure_delete = ON;",
		} {
			res := m.Exec(v)
			err := res.Error
			if err != nil {
				log.Fatal(err)
			}
		}
	}
	return m.DB
}

func (m *MetaV2) init() error {
	tables := []interface{}{MetaInfoV2{}}
	var errs error
	for _, v := range tables {
		err := m.db().AutoMigrate(v)
		if err != nil {
			errs = errors.Join(errs, err)
		}
	}
	return errs
}

type MetaV2 struct {
	*gorm.DB
	root  string
	dbDir string
}

func NewMetaV2(root, dbDir string) *MetaV2 {
	m := MetaV2{
		root:  root,
		dbDir: dbDir,
	}
	err := m.init()
	if err != nil {
		log.Println(err)
	}
	return &m
}

func (m *MetaV2) LoadPath(relPath string) (*MetaInfoV2, error) {
	info, err := os.Stat(filepath.Join(m.root, relPath))
	if err != nil {
		return nil, err
	}
	go m.IndexDynamicly(relPath)
	i, _, err := m.NewInfo(relPath, info)
	return i, err
}

// MoveDir("Downloads/xxx", ".Trash") -> move all files in Downloads/xxx to .Trash/xxx
func (m *MetaV2) MoveDir(srcDir, dstDir string) error {
	srcDir = strings.TrimLeft(srcDir, "/")
	dstDir = strings.TrimLeft(dstDir, "/")

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

func (m *MetaV2) NewInfo(path string, info os.FileInfo) (*MetaInfoV2, bool, error) {
	if info == nil {
		return nil, false, errors.New("file not exist")
	}
	path = strings.TrimLeft(path, "/")
	i, err := m.Get(path)
	if err != nil {
		// file not exist
		var dir string
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
		return i, true, nil
	}
	if i.ModTime.Equal(info.ModTime()) && i.Size == info.Size() {
		return i, false, nil
	} else {
		if i.ModTime.Equal(info.ModTime()) {
			log.Println("update", path, i.Size, "->", info.Size())
		} else if i.Size == info.Size() {
			log.Println("update", path, i.ModTime.Format(time.RFC3339), info.ModTime().Format(time.RFC3339))
		} else {
			log.Println("update", path, i.ModTime.Format(time.RFC3339), "->", info.ModTime().Format(time.RFC3339), i.Size, "->", info.Size())
		}
		i.Size = info.Size()
		i.ModTime = info.ModTime()
		m.db().Updates(i)
		return i, true, nil
	}
}

func (m *MetaV2) Get(path string) (*MetaInfoV2, error) {
	path = strings.TrimLeft(path, "/")
	info := MetaInfoV2{
		Path:   path,
		MetaV2: m,
	}
	res := m.db().First(&info)
	if res.Error != nil {
		return nil, res.Error
	}
	return &info, nil
}

type MetaInfoV2 struct {
	Path    string         `json:"path" xorm:"pk" gorm:"primaryKey"`
	Dir     string         `json:"dir" xorm:"index" gorm:"index"`
	Size    int64          `json:"size"`
	ModTime time.Time      `json:"mod_time"`
	Label   string         `json:"label"`
	Tags    datatypes.JSON `json:"tags"`
	Star    bool           `json:"star"`
	Icons   datatypes.JSON `json:"icons"`
	OldLoc  string
	Context datatypes.JSON
	MetaV2  *MetaV2 `xorm:"-" gorm:"-"`
}

// SetContext sets the context map to the Context field
func (m *MetaInfoV2) SetContext(ctx map[string]interface{}) error {
	bytes, err := json.Marshal(ctx)
	if err != nil {
		return err
	}
	m.Context = datatypes.JSON(bytes)
	return nil
}

// GetContext returns the context map from the Context field
func (m *MetaInfoV2) GetContext() map[string]interface{} {
	var ctx map[string]interface{}
	if err := json.Unmarshal(m.Context, &ctx); err != nil {
		return nil
	}
	return ctx
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
		err := filepath.Walk(filepath.Join(m.root, prefix), func(path string, info os.FileInfo, err error) error {
			path, err = filepath.Rel(m.root, path)
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

// index a path possible still in progress
func (m *MetaV2) IndexDynamicly(prefixs ...string) error {
	if len(prefixs) == 0 {
		prefixs = append(prefixs, "")
	}
	var recheck bool
	for _, prefix := range prefixs {
		prefix = strings.TrimLeft(prefix, "/")
		err := filepath.Walk(filepath.Join(m.root, prefix), func(path string, info os.FileInfo, err error) error {
			path, err = filepath.Rel(m.root, path)
			if err != nil {
				return err
			}
			_, changed, _ := m.NewInfo(path, info)
			if changed {
				recheck = true
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	if recheck {
		time.Sleep(10 * time.Second)
		for _, prefix := range prefixs {
			prefix = strings.TrimLeft(prefix, "/")
			Cache.Remove("size:" + prefix)
		}
		return m.IndexDynamicly(prefixs...)
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
			if _, err := os.Stat(filepath.Join(m.root, i.Path)); err != nil {
				m.Del(i.Path)
			}
		}
	}
	return nil
}

func (m *MetaV2) CacheSize() error {
	var infos MetaInfoV2s
	res := m.db().Distinct("dir").Find(&infos)
	if res.Error != nil {
		return res.Error
	}
	for _, i := range infos {
		size, err := m.Size(i.Dir)
		if err != nil {
			return err
		}
		Cache.SetWithExpire("size:"+i.Dir, size, time.Hour)
	}

	return nil
}

func (m *MetaV2) Close() error {
	m.DB = nil
	return nil
}

type MetaV2ListOptions struct {
	Prefix *string
}

func (m *MetaV2) List(opts MetaV2ListOptions) (MetaInfoV2s, error) {
	var list MetaInfoV2s
	session := m.db().Session(&gorm.Session{})
	if opts.Prefix != nil {
		session = session.Where("path like ?", *opts.Prefix+"%")
	}
	res := session.Find(&list)
	return list, res.Error
}

func (m *MetaV2) Size(prefix string) (float64, error) {
	var sumSize int64
	m.db().Model(MetaInfoV2{}).Select("SUM(size) as size").Where("path LIKE ?", prefix+"%").Find(&sumSize)
	if sumSize == 0 {
		// m.Index(prefix)
		go m.IndexDynamicly(prefix)
		time.Sleep(100 * time.Millisecond)
		m.db().Model(MetaInfoV2{}).Select("SUM(size) as size").Where("path LIKE ?", prefix+"%").Find(&sumSize)
		return float64(sumSize), nil
	}
	return float64(sumSize), nil
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
	val.Path = strings.TrimLeft(val.Path, "/")
	val.Dir = strings.TrimLeft(val.Dir, "/")
	_, err := m.Get(val.Path)
	if err != nil {
		m.db().Create(val)
	} else {
		m.db().Updates(val)
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
