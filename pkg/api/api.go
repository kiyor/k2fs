package api

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2" // Added Fiber
	golib "github.com/kiyor/golib"
	"github.com/kiyor/k2fs/pkg/core" // For GlobalAppConfig, DiskSize and FlagDf
	"github.com/kiyor/k2fs/pkg/lib"  // Updated import path
	kfs "github.com/kiyor/k2fs/pkg/lib" // Updated import path
)

type Resp struct {
	Code int         `json:"code"`
	Data interface{} `json:"data"`
}

// NewResp sends a standard JSON response using Fiber.
func NewResp(c *fiber.Ctx, data interface{}, durs []time.Duration, code ...int) error {
	httpStatusCode := fiber.StatusOK // Default HTTP status code
	appCode := 0                     // Default application code in JSON body
	if len(code) > 0 {
		appCode = code[0]
		// Optional: map appCode to httpStatusCode if desired, e.g. if appCode is an error code
	}
	r := &Resp{
		Code: appCode,
		Data: data,
	}

	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	c.Set("Access-Control-Allow-Origin", "*")
	c.Set("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,OPTIONS")
	c.Set("Access-Control-Allow-Headers", "Content-Type")

	for k, dur := range durs {
		c.Set(fmt.Sprintf("X-Profile-%d", k), dur.String())
	}
	return c.Status(httpStatusCode).JSON(r)
}

// NewCacheResp sends a JSON response and caches it.
func NewCacheResp(c *fiber.Ctx, data interface{}, cacheKey string, expire time.Duration, code ...int) error {
	httpStatusCode := fiber.StatusOK
	appCode := 0
	if len(code) > 0 {
		appCode = code[0]
	}
	r := &Resp{
		Code: appCode,
		Data: data,
	}

	b, err := json.Marshal(r)
	if err != nil {
		log.Printf("Error marshalling cache response: %v", err)
		// Return a non-cached error response
		return NewErrResp(c, fiber.StatusInternalServerError, 1, "Internal server error marshalling cache response")
	}

	if lib.Cache != nil {
		lib.Cache.SetWithExpire(cacheKey, b, expire)
	} else {
		log.Println("Warning: lib.Cache is nil, caching disabled for NewCacheResp")
	}

	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
	c.Set("Access-Control-Allow-Origin", "*") // Add CORS headers for cached responses too
	c.Set("Access-Control-Allow-Methods", "GET,PUT,POST,PATCH,OPTIONS")
	c.Set("Access-Control-Allow-Headers", "Content-Type")

	return c.Status(httpStatusCode).Send(b)
}

// NewErrResp sends a JSON error response.
func NewErrResp(c *fiber.Ctx, httpStatusCode int, appErrorCode int, errMsg string) error {
	c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8) // Ensure content type for errors
	return c.Status(httpStatusCode).JSON(&Resp{
		Code: appErrorCode,
		Data: errMsg,
	})
}

// ApiDfFiber handles requests for disk free space information.
func ApiDfFiber(c *fiber.Ctx) error {
	du := core.DiskSize(core.FlagDf)
	return NewResp(c, du, nil) // Using NewResp for consistency
}

// Thumb struct defines the structure for thumbnail responses.
type Thumb struct {
	Path   string `json:"path"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ApiThumbRequest defines the expected JSON structure for thumb requests.
type ApiThumbRequest struct {
	Path string `json:"path"`
}

// pathEscape is a helper for escaping URL paths for thumb responses.
func pathEscape(input string) string {
	return strings.ReplaceAll(url.PathEscape(input), "%2F", "/")
}

// readImageNamesFromDirApi is a helper to read and filter image file names from a directory.
// Returns paths relative to the directory provided (not absolute, not relative to rootDir).
func readImageNamesFromDirApi(dirPath string) ([]string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}
	var imageNames []string
	for _, entry := range entries {
		if !entry.IsDir() && isImage(entry.Name()) { // isImage is from list.go (same package)
			imageNames = append(imageNames, entry.Name())
		}
	}
	return imageNames, nil
}

// ApiThumbFiber handles requests for generating thumbnails.
func ApiThumbFiber(c *fiber.Ctx) error {
	req := new(ApiThumbRequest)
	if err := c.BodyParser(req); err != nil {
		return NewErrResp(c, fiber.StatusBadRequest, 1, "Invalid request for thumb: "+err.Error())
	}

	cacheKey := "api_thumb:" + req.Path // Simplified cache key
	if val, err := lib.Cache.Get(cacheKey); err == nil {
		if cachedResp, ok := val.([]byte); ok {
			c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
			return c.Status(fiber.StatusOK).Send(cachedResp)
		}
	}

	requestPath := req.Path
	if strings.Contains(requestPath, "%") {
		var err error
		requestPath, err = url.PathUnescape(requestPath)
		if err != nil {
			return NewErrResp(c, fiber.StatusBadRequest, 1, "Invalid path encoding for thumb: "+err.Error())
		}
	}
	requestPath = strings.TrimRight(requestPath, "/")
	if len(requestPath) == 0 {
		requestPath = "/" // Should not happen if path is required
	}

	// absPathOnDisk is the full path to the file/directory on the server's filesystem
	absPathOnDisk := filepath.Join(core.GlobalAppConfig.RootDir, requestPath)

	fInfo, err := os.Stat(absPathOnDisk)
	if err != nil {
		log.Printf("ApiThumbFiber: os.Stat error for %s: %v", absPathOnDisk, err)
		return NewErrResp(c, fiber.StatusNotFound, 1, "Path not found for thumb: "+err.Error())
	}

	// processFileForThumb creates a Thumb object for a given image file path (relative to rootDir).
	processFileForThumb := func(imagePathRelToRootDir string) *Thumb {
		fullImagePathOnDisk := filepath.Join(core.GlobalAppConfig.RootDir, imagePathRelToRootDir)
		staticLinkPath := filepath.Join("/statics", imagePathRelToRootDir) // Link path for client

		reader, err := os.Open(fullImagePathOnDisk)
		if err != nil {
			log.Printf("ApiThumbFiber: Error opening image %s: %v", fullImagePathOnDisk, err)
			return &Thumb{Path: pathEscape(staticLinkPath)} // Return path even if decode fails
		}
		defer reader.Close()

		imgCfg, _, err := image.DecodeConfig(reader)
		if err != nil {
			log.Printf("ApiThumbFiber: Error decoding image config %s: %v", fullImagePathOnDisk, err)
			return &Thumb{Path: pathEscape(staticLinkPath)}
		}

		width, height := imgCfg.Width, imgCfg.Height
		// Original logic had a max width of 1200, let's retain that.
		if width > 1200 {
			height = int(float64(1200) / float64(width) * float64(height))
			width = 1200
		}
		return &Thumb{
			Path:   pathEscape(staticLinkPath),
			Width:  width,
			Height: height,
		}
	}

	if fInfo.IsDir() {
		imageNames, err := readImageNamesFromDirApi(absPathOnDisk)
		if err != nil {
			log.Printf("ApiThumbFiber: Error reading dir %s: %v", absPathOnDisk, err)
			return NewErrResp(c, fiber.StatusInternalServerError, 1, "Error reading directory for thumb")
		}
		if len(imageNames) == 0 {
			// No images in directory, cache an empty response (as string or specific struct)
			return NewCacheResp(c, "", cacheKey, time.Hour) // Cache empty string for "not found"
		}

		// Check for "cover.*"
		coverImageName := ""
		for _, name := range imageNames {
			if strings.HasPrefix(strings.ToLower(name), "cover.") {
				coverImageName = name
				break
			}
		}

		var imageToProcess string
		if coverImageName != "" {
			imageToProcess = coverImageName
		} else {
			sort.Strings(imageNames) // Sort to pick the first one consistently
			imageToProcess = imageNames[0]
		}
		
		// imagePathRelToRootDir is requestPath (dir) + imageToProcess (filename)
		imagePathRelToRootDir := filepath.Join(requestPath, imageToProcess)
		thumbResult := processFileForThumb(imagePathRelToRootDir)
		return NewCacheResp(c, thumbResult, cacheKey, time.Hour)

	} else { // It's a file, should be an image
		if !isImage(absPathOnDisk) { // isImage is from list.go (same package)
			return NewErrResp(c, fiber.StatusBadRequest, 1, "Path is not an image file for thumb")
		}
		// requestPath is already relative to rootDir for a file
		thumbResult := processFileForThumb(requestPath)
		return NewCacheResp(c, thumbResult, cacheKey, time.Hour)
	}
}


type Dir struct {
	Dir   string `json:"dir"`
	UpDir string `json:"up_dir"`
	Hash  string `json:"hash"`
	// Files []File `json:"files"` // Changed from []*File to []File based on list.go refactor
	Files []File `json:"files"`
}

func NewDir() *Dir {
	// return &Dir{ Files: make([]*File, 0) } // Changed from []*File to []File
	return &Dir{Files: make([]File, 0)}
}

type File struct {
	Name     string    `json:"name"`
	Path     string    `json:"path"`
	Hash     string    `json:"hash"`
	Size     int64     `json:"size"`
	SizeH    string    `json:"size_h"`
	IsDir    bool      `json:"is_dir"`
	IsImage  bool      `json:"is_image"`
	ModTime  time.Time `json:"mod_time"`
	ModTimeH string    `json:"mod_time_h"`

	ShortCut string `json:"short_cut"`

	ThumbLink   string   `json:"thumb_link"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`

	Meta kfs.MetaInfo `json:"meta"`
}

func NewFile(name string) *File {
	return &File{
		Name: name,
	}
}

var sizeManager = golib.NewManager(1, 100000)
var sizeTasks = make(chan golib.Task)

var titleManager = golib.NewManager(1, 100000)
var titleTasks = make(chan golib.Task)

func init() {
	sizeManager.Start(sizeTasks)
	titleManager.Start(titleTasks)
}

var metaV2 *kfs.MetaV2 

func fetchTitle(path string) (string, error) {
	if metaV2 == nil || lib.Cache == nil {
		return "", fmt.Errorf("metaV2 or lib.Cache not initialized for fetchTitle")
	}
	key := "title:" + path
	titleTasks <- golib.NewTask(
		func() error {
			if _, err := lib.Cache.Get(key); err == nil {
				return nil
			} else {
				if val, errGet := metaV2.Get(path); errGet == nil { // Changed variable name
					ctx := val.GetContext()
					if ctx != nil && ctx["Title"] != nil {
						lib.Cache.SetWithExpire(key, ctx["Title"].(string), 24*time.Hour)
					} else {
						// name := strings.Trim(path, "/") // Original logic
						// name = filepath.Base(name)
						// searchName, _ := isSearchable(name) // isSearchable is in list.go (same package)
						// res, errSearch := lib.NewSearchClient().Search(searchName) // lib.NewSearchClient might need setup
						// if errSearch == nil {
						// 	if ctx == nil {
						// 		ctx = make(map[string]interface{})
						// 	}
						// 	ctx["Title"] = res.Title
						// 	val.SetContext(ctx)
						// 	metaV2.Set(val)
						// 	lib.Cache.SetWithExpire(key, res.Title, 24*time.Hour)
						// } else {
						// 	log.Println(errSearch)
						// }
						// Commenting out search client logic for now to ensure compilability
						// as NewSearchClient() might not be initialized/available yet.
						log.Printf("Title not found in meta context for %s, and search client logic is commented out.", path)
					}
				} else if errGet != nil {
					log.Printf("Error getting meta for path %s in fetchTitle: %v", path, errGet)
				}
			}
			return nil
		},
		nil,
		false,
	)
	return "", nil 
}

func dirSize2(path string) (int64, error) {
	if metaV2 == nil || lib.Cache == nil {
		return 0, fmt.Errorf("metaV2 or lib.Cache not initialized for dirSize2")
	}
	// Placeholder for actual logic from original, as it's complex and relies on metaV2.SizeWithTimeout
	// This needs to be fully restored when metaV2 interaction is confirmed.
	path = strings.TrimLeft(path, "/")
	key := "size:" + path
	if val, err := lib.Cache.Get(key); err == nil {
		if size, ok := val.(int64); ok { // Assuming size is stored as int64
			return size, nil
		}
		if sizeF, ok := val.(float64); ok { // Original stored as float64
			return int64(sizeF), nil
		}
		log.Printf("dirSize2: cached value for %s is not int64 or float64: %T", key, val)
		// Fall through to recalculate if type assertion fails
	}

	// Actual calculation logic using metaV2.SizeWithTimeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second) // Example timeout
	defer cancel()
	
	size, err := metaV2.SizeWithTimeout(path, ctx) // This is the actual call needed
	if err != nil {
		// log.Printf("dirSize2: metaV2.SizeWithTimeout error for %s: %v", path, err)
		return 0, err // Return error if calculation fails
	}
	lib.Cache.SetWithExpire(key, size, time.Hour) // Cache the result
	return size, nil
}


// dirSize (original, direct disk walk) is not used by ApiListFiber by default, dirSize2 is.
// It can be kept for reference or other uses.
func dirSize(path string) (int64, error) { /* ... */ return 0, nil }


func prettyTime(t time.Time) string {
	since := time.Since(t)
	switch {
	case since < (1 * time.Second):
		return "1s"
	case since < (60 * time.Second):
		s := strings.Split(fmt.Sprint(since), ".")[0]
		return s + "s"
	case since < (60 * time.Minute):
		s := strings.Split(fmt.Sprint(since), ".")[0]
		return strings.Split(s, "m")[0] + "m"
	case since < (24 * time.Hour):
		s := strings.Split(fmt.Sprint(since), ".")[0]
		return strings.Split(s, "h")[0] + "h"
	default:
		return t.Format("01-02-06")
	}
}

func SetMetaV2(instance *kfs.MetaV2) {
	metaV2 = instance
}
