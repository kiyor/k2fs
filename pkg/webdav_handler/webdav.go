package webdav_handler

import (
	"net/http"
	// "strings" // Removed unused import

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/adaptor/v2" // For Fiber to net/http adapter
	"golang.org/x/net/webdav"
	"github.com/kiyor/k2fs/pkg/core" // For core.GlobalAppConfig.RootDir
)

// NewHandler creates a Fiber handler for serving WebDAV.
func NewHandler() fiber.Handler {
	// Initialize the WebDAV handler
	davHandler := &webdav.Handler{
		// Prefix:     "/", // The prefix is handled by Fiber's routing group.
		// The path passed to davHandler.ServeHTTP will be relative to the group path.
		// For example, if Fiber group is /webdav, and request is /webdav/file,
		// then r.URL.Path for davHandler will be /file.
		// So, Prefix should be "/" if Fiber group handles "/webdav".
		Prefix:     "/", 
		FileSystem: webdav.Dir(core.GlobalAppConfig.RootDir), // Access RootDir via shared config
		LockSystem: webdav.NewMemLS(),                       // In-memory lock system
	}

	// Create an http.HandlerFunc that wraps davHandler.ServeHTTP
	// and explicitly handles OPTIONS requests for WebDAV.
	httpHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The r.URL.Path here is the path *after* the Fiber group path has been stripped.
		// So if Fiber group is /webdav, and request is /webdav/ or /webdav/foo,
		// r.URL.Path will be / or /foo respectively.
		// The original check `strings.HasPrefix(r.URL.Path, "/")` is always true here.
		// The key is that this OPTIONS handler is only reached for /webdav paths due to Fiber routing.
		if r.Method == http.MethodOptions {
			w.Header().Set("DAV", "1, 2") // Standard DAV header
			// Allow header lists methods supported by WebDAV
			w.Header().Set("Allow", "OPTIONS, GET, HEAD, POST, DELETE, PROPFIND, PROPPATCH, COPY, MOVE, LOCK, UNLOCK")
			w.WriteHeader(http.StatusOK)
			return
		}
		// For other methods, let the WebDAV handler process the request.
		davHandler.ServeHTTP(w, r)
	})

	// Convert the http.HandlerFunc to a Fiber handler using the adaptor.
	return adaptor.HTTPHandler(httpHandler)
}
