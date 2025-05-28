package main

import (
	_ "embed"
	"fmt"
	"html/template" // Added for app.js template execution
	"log"
	"os"
	"path/filepath" // For Trash path
	"regexp"
	// "runtime" // Commented out for now, was for GOMAXPROCS
	"strings" // For template parsing, if needed by genSlice or others
	"sync"    // For shortMu, if still used directly in this file
	// "text/template" // html/template is more specific for HTML
	"time"

	_ "net/http/pprof" // Keep for now

	"github.com/spf13/cobra" // Added Cobra

	"github.com/gofiber/fiber/v2"         // Added Fiber
	"github.com/gofiber/template/html/v2" // Added Fiber HTML template engine
	
	// K2FS specific packages
	"github.com/kiyor/k2fs/pkg/api"  // Added for API handlers
	"github.com/kiyor/k2fs/pkg/core" // Added for core types/vars like core.FlagDf, core.GlobalAppConfig
	"github.com/kiyor/k2fs/pkg/lib"  // Added for lib.InitRedisPool, lib.NewMetaV2
	"github.com/kiyor/k2fs/pkg/webdav_handler" // Added for WebDAV
)

var (
	addr               string
	rootDir, dbDir     string // These are populated by Cobra flags
	intf               string
	port               string
	flagHost           string
	flagStaticFileHost string
	metaHost           string
	redisHostCmd       string // Variable to hold the redis-host flag value

	//go:embed tmpl.html
	basicView string // This is the content of tmpl.html

	//go:embed app.js
	appjs string
	//go:embed bootstrap.css
	bootstrapcss string

	// flagDf is now handled by core.FlagDf directly with StringSliceVar
)

const (
	// APP name
	APP = "k2fs" // Used by original session store, might be needed for Fiber session key
)

var (
	reIpad  = regexp.MustCompile(` Version/\d+\.\d+`)
	rePhone = regexp.MustCompile(`(P|p)hone`)
	reIos   = regexp.MustCompile(`\((iPhone|iPad);`)
)

// reqToMapFiber adapts the original req2map logic for Fiber context
func reqToMapFiber(c *fiber.Ctx) map[string]interface{} {
	scheme := "http"
	if c.Secure() {
		scheme = "https"
	}
	fullHost := string(c.Request().Host()) 
	u := scheme + "://" + fullHost

	if len(flagHost) > 0 { // flagHost is populated by Cobra
		u = flagHost
	}

	m := make(map[string]interface{})
	m["host"] = u
	m["ios"] = reIos.MatchString(c.Get(fiber.HeaderUserAgent))
	m["phone"] = rePhone.MatchString(c.Get(fiber.HeaderUserAgent))
	m["metahost"] = metaHost // metaHost is populated by Cobra
	return m
}

// serveAppJS handles serving the embedded app.js, processed as a template.
func serveAppJS(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "application/javascript")
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")

	// Note: Using html/template here for app.js. If app.js doesn't contain HTML constructs
	// and only needs text replacement, text/template might be slightly more performant,
	// but html/template is generally safer if there's any chance of HTML-like sequences.
	t, err := template.New("app.js").Delims("[[", "]]").Funcs(
		template.FuncMap{
			"slice": genSlice, 
		},
	).Parse(appjs)

	if err != nil {
		log.Printf("Error parsing app.js template: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}

	templateData := reqToMapFiber(c)
    var buf strings.Builder
	err = t.Execute(&buf, templateData)
    if err != nil {
        log.Printf("Error executing app.js template: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
    }
	return c.SendString(buf.String())
}

// serveBootstrapCSS handles serving the embedded bootstrap.css.
func serveBootstrapCSS(c *fiber.Ctx) error {
	c.Set(fiber.HeaderContentType, "text/css")
	c.Set(fiber.HeaderCacheControl, "public, max-age=300")
	return c.SendString(bootstrapcss)
}

// genSlice is a utility function used by templates (appjs and tmpl.html).
func genSlice(i ...interface{}) chan interface{} {
	o := make(chan interface{})
	go func() {
		for _, v := range i {
			o <- v
		}
		close(o)
	}()
	return o
}

// ServeUniversalFiber serves the main application page (tmpl.html).
func ServeUniversalFiber(c *fiber.Ctx) error {
	if c.Path() == "/favicon.ico" {
		// You might want to serve an actual favicon here if you have one
		// For now, just sending a 200 OK to prevent errors in logs for favicon requests
		return c.SendStatus(fiber.StatusOK)
	}

	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	
	// The data for the template
	templateData := reqToMapFiber(c)

	// Render the embedded basicView (tmpl.html) string as a template.
	// Fiber's c.Render() is for file-based templates loaded by the engine.
	// To render an embedded string template, we use html/template directly.
	// The engine configured with Delims and AddFunc is for c.Render("filename.html", ...).
	// If tmpl.html is meant to be a file on disk, then c.Render("tmpl.html", templateData) would work.
	// Since basicView is an embedded string, we parse and execute it directly.
	
	tmpl, err := template.New("tmpl.html").Delims("[[", "]]").Funcs(
		template.FuncMap{
			"slice": genSlice,
		},
	).Parse(basicView)

	if err != nil {
		log.Printf("Error parsing tmpl.html template: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	
	// Execute template into a buffer first to catch execution errors and then send
	var buf strings.Builder
	if err := tmpl.Execute(&buf, templateData); err != nil {
		log.Printf("Error executing tmpl.html template: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}
	return c.SendString(buf.String())
}


// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "k2fs",
	Short: "A browsable file server.",
	Run: func(cmd *cobra.Command, args []string) {
		// Populate core.GlobalAppConfig.RedisHost first
		core.GlobalAppConfig.RedisHost = redisHostCmd 

		// Initialize Redis Pool (for lib.Cache) - Now uses core.GlobalAppConfig.RedisHost
		lib.InitRedisPool() 

		var err error
		rootDir, err = filepath.Abs(rootDir)
		if err != nil {
			log.Fatalf("Error getting absolute path for rootDir: %v", err)
		}
		dbDir, err = filepath.Abs(dbDir)
		if err != nil {
			log.Fatalf("Error getting absolute path for dbDir: %v", err)
		}
		
		core.GlobalAppConfig.RootDir = rootDir
		core.GlobalAppConfig.FlagHost = flagHost 
		core.GlobalAppConfig.MetaHost = metaHost 
		core.GlobalAppConfig.TrashPath = filepath.Join(rootDir, ".Trash")

		if _, statErr := os.Stat(core.GlobalAppConfig.TrashPath); os.IsNotExist(statErr) {
			if mkErr := os.MkdirAll(core.GlobalAppConfig.TrashPath, 0755); mkErr != nil {
				log.Fatalf("Error creating .Trash directory at %s: %v", core.GlobalAppConfig.TrashPath, mkErr)
			}
			log.Printf(".Trash directory created at: %s", core.GlobalAppConfig.TrashPath)
		}
		
		metaV2Instance := lib.NewMetaV2(core.GlobalAppConfig.RootDir, dbDir) 
		api.SetMetaV2(metaV2Instance) 

		// Initialize Fiber HTML template engine
		// The directory specified here is for file-based templates.
		// Embedded templates (like basicView, appjs) are handled separately if not written to disk.
		// photo.html and player.html are assumed to be files in the "./" directory (relative to CWD).
		engine := html.New("./", ".html") 
		engine.Delims("[[", "]]")      // Set custom delimiters for file-based templates
		engine.AddFunc("slice", genSlice) // Add custom 'slice' function for file-based templates

		app := fiber.New(fiber.Config{
			Views: engine, // This engine is used for c.Render("filename.html", ...)
		})

		if _, err := os.Stat(core.GlobalAppConfig.RootDir); os.IsNotExist(err) { 
			log.Fatalf("Root directory '%s' does not exist. Please specify with --root flag.", core.GlobalAppConfig.RootDir)
		}
		app.Static("/statics", core.GlobalAppConfig.RootDir) 
		app.Static("/.local", "./local")
		
		app.Get("/app.js", serveAppJS)
		app.Get("/bootstrap.css", serveBootstrapCSS)

		// API routes
		apiGroup := app.Group("/api")
		apiGroup.Get("/df", api.ApiDfFiber)
		apiGroup.Post("/list", api.ApiListFiber)
		apiGroup.Post("/thumb", api.ApiThumbFiber)
		apiGroup.Get("/session", api.ApiSessionFiber) 
		apiGroup.Post("/operation", api.ApiOperationFiber)

		// WebDAV Route
		webdavGroup := app.Group("/webdav")
		webdavGroup.Use(webdav_handler.NewHandler())

		// Page rendering routes
		app.Get("/photo/*", api.RenderPhotoFiber)   // Will be created in pkg/api/photo.go
		app.Get("/player", api.RenderPlayerFiber) // Will be created in pkg/api/player.go
		
		// Catch-all for serving the main UI (tmpl.html)
		// This MUST be the last route registered for GET requests on /*
		app.Get("/*", ServeUniversalFiber)


		addr = intf + port 
		
		fmt.Printf("K2FS server starting with Fiber on address: %s, serving /statics from %s\n", addr, core.GlobalAppConfig.RootDir)
		log.Fatal(app.Listen(addr))
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}

var shortUrl map[string]string 
var shortMu sync.Mutex         

func init() {
	rootCmd.PersistentFlags().StringVarP(&intf, "interface", "i", "0.0.0.0", "http service interface address")
	rootCmd.PersistentFlags().StringVarP(&port, "listen", "l", ":8080", "http service listen port")
	rootCmd.PersistentFlags().StringVar(&rootDir, "root", ".", "root dir for /statics")
	rootCmd.PersistentFlags().StringVar(&dbDir, "db", ".", "db dir")
	rootCmd.PersistentFlags().StringVar(&flagHost, "host", "", "host if need overwrite; syntax like http://a.com(:8080)")
	rootCmd.PersistentFlags().StringVar(&flagStaticFileHost, "static", "", "static file host like http://a.com(:8080)")
	rootCmd.PersistentFlags().StringVar(&metaHost, "meta", "10.43.1.10", "meta host")
	rootCmd.PersistentFlags().StringVar(&redisHostCmd, "redis-host", "localhost:6379", "Redis host address (e.g. localhost:6379)")
	rootCmd.PersistentFlags().StringSliceVar(&core.FlagDf, "df", []string{}, "monitor mount dir (can be specified multiple times)")
}
