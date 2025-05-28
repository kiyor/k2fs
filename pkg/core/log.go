package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http" // This will need to be adapted if used with Fiber
	"os"
	"time"

	"github.com/kiyor/terminal/color"
)

// LogHandler is a custom HTTP log handler.
// NOTE: This was designed for net/http. Fiber has its own logging middleware.
// This may need refactoring or may be deprecated.
type LogHandler struct {
	l *log.Logger
}

// NewLogHandler creates a new LogHandler.
func NewLogHandler() *LogHandler {
	return &LogHandler{
		l: log.New(os.Stdout, color.Sprint("@{g}[http]@{|} "), log.LstdFlags),
	}
}

type statusWriter struct {
	http.ResponseWriter
	http.Flusher
	http.Hijacker
	status int
	length int
	body   []byte
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	w.length += len(b)
	w.body = append(w.body, b...)

	return w.ResponseWriter.Write(b)
}

// Set allows configuring the logger's output, prefix, and flags.
func (l *LogHandler) Set(out io.Writer, prefix string, flag int) {
	l.l = log.New(out, prefix, flag)
}

// Handler wraps an http.Handler to log requests.
// NOTE: This is for net/http.
func (l *LogHandler) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t1 := time.Now()
		ctx := context.Background()

		writer := statusWriter{
			w,
			w.(http.Flusher),
			w.(http.Hijacker),
			0,
			0,
			nil,
		}
		r = r.WithContext(ctx)
		next.ServeHTTP(&writer, r)

		reqURI := r.URL.Path
		if len(r.URL.Query()) > 0 {
			reqURI += "?" + r.URL.Query().Encode()
		}
		ua := r.Header.Get("User-Agent")
		res := fmt.Sprintf("%v %v %v %v %v %v '%v'", r.RemoteAddr, writer.status, writer.length, r.Method, reqURI, time.Since(t1), ua)
		l.l.Println(res)
	})
}
