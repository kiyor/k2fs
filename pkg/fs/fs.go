// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// HTTP file system request handler

package fs // Corrected package declaration

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs" // Standard library fs
	"log"
	"mime"
	"mime/multipart"
	"net/http" // This package still uses net/http types heavily.
	"net/textproto"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disintegration/imaging"
)

var tmpdir = "/tmp/thumb"

func init() {
	os.MkdirAll(tmpdir, 0755)
}

// A Dir implements FileSystem using the native file system restricted to a
// specific directory tree.
//
// While the FileSystem.Open method takes '/'-separated paths, a Dir's string
// value is a filename on the native file system, not a URL, so it is separated
// by filepath.Separator, which isn't necessarily '/'.
//
// Note that Dir could expose sensitive files and directories. Dir will follow
// symlinks pointing out of the directory tree, which can be especially dangerous
// if serving from a directory in which users are able to create arbitrary symlinks.
// Dir will also allow access to files and directories starting with a period,
// which could expose sensitive directories like .git or sensitive files like
// .htpasswd. To exclude files with a leading period, remove the files/directories
// from the server or create a custom FileSystem implementation.
//
// An empty Dir is treated as ".".
type Dir string

// mapOpenError maps the provided non-nil error from opening name
// to a possibly better non-nil error. In particular, it turns OS-specific errors
// about opening files in non-directories into fs.ErrNotExist. See Issues 18984 and 49552.
func mapOpenError(originalErr error, name string, sep rune, stat func(string) (fs.FileInfo, error)) error {
	if errors.Is(originalErr, fs.ErrNotExist) || errors.Is(originalErr, fs.ErrPermission) {
		return originalErr
	}

	parts := strings.Split(name, string(sep))
	for i := range parts {
		if parts[i] == "" {
			continue
		}
		fi, err := stat(strings.Join(parts[:i+1], string(sep)))
		if err != nil {
			return originalErr
		}
		if !fi.IsDir() {
			return fs.ErrNotExist
		}
	}
	return originalErr
}

// Open implements FileSystem using os.Open, opening files for reading rooted
// and relative to the directory d.
func (d Dir) Open(name string) (http.File, error) { // Returns http.File, not fs.File
	if filepath.Separator != '/' && strings.ContainsRune(name, filepath.Separator) {
		return nil, errors.New("http: invalid character in file path")
	}
	dir := string(d)
	if dir == "" {
		dir = "."
	}
	fullName := filepath.Join(dir, filepath.FromSlash(path.Clean("/"+name)))
	f, err := os.Open(fullName)
	if err != nil {
		return nil, mapOpenError(err, fullName, filepath.Separator, os.Stat)
	}
	return f, nil
}

// A FileSystem implements access to a collection of named files.
// The elements in a file path are separated by slash ('/', U+002F)
// characters, regardless of host operating system convention.
// See the FileServer function to convert a FileSystem to a Handler.
//
// This interface predates the fs.FS interface, which can be used instead:
// the FS adapter function converts an fs.FS to a FileSystem.
// type FileSystem interface { // This is http.FileSystem
// 	Open(name string) (http.File, error)
// }

// A File is returned by a FileSystem's Open method and can be
// served by the FileServer implementation.
//
// The methods should behave the same as those on an *os.File.
// type File interface { // This is http.File
// 	io.Closer
// 	io.Reader
// 	io.Seeker
// 	Readdir(count int) ([]fs.FileInfo, error)
// 	Stat() (fs.FileInfo, error)
// }


// Helper functions like dirList, ServeContent, serveFile etc. are direct copies
// from net/http/fs.go and are tightly coupled with net/http types.
// Their direct inclusion in a 'package fs' might be misleading if the goal is a generic fs package.
// However, per task, only package name is changed.

// For brevity, the rest of the functions (dirList, ServeContent, etc.) are kept as is,
// but they use http.ResponseWriter, http.Request, http.File, etc.
// This means this 'package fs' is essentially a http file server utility package.

// logf logs to the INFO log.
// Deprecated: We don't do this anymore.
func logf(r *http.Request, format string, args ...interface{}) {
	// Original: log.Printf("http: "+format, args...)
	// For now, just use standard log
	log.Printf("fs: "+format, args...)
}

// Error replies to the request with the specified error message and HTTP code.
// It does not otherwise end the request; the caller should ensure no further
// writes are done to w.
// The error message should be plain text.
func Error(w http.ResponseWriter, error string, code int) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	fmt.Fprintln(w, error)
}

var htmlReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	// "&#34;" is possibly more compatible than "&quot;".
	`"`, "&#34;",
	// "&#39;" is possibly more compatible than "&apos;" and "\'".
	"'", "&#39;",
)


const sniffLen = 512


type anyDirs interface {
	len() int
	name(i int) string
	isDir(i int) bool
}

type fileInfoDirs []fs.FileInfo

func (d fileInfoDirs) len() int          { return len(d) }
func (d fileInfoDirs) isDir(i int) bool  { return d[i].IsDir() }
func (d fileInfoDirs) name(i int) string { return d[i].Name() }

type dirEntryDirs []fs.DirEntry

func (d dirEntryDirs) len() int          { return len(d) }
func (d dirEntryDirs) isDir(i int) bool  { return d[i].IsDir() }
func (d dirEntryDirs) name(i int) string { return d[i].Name() }

func dirList(w http.ResponseWriter, r *http.Request, f http.File) {
	var dirs anyDirs
	var err error
	if d, ok := f.(fs.ReadDirFile); ok {
		var list dirEntryDirs
		list, err = d.ReadDir(-1)
		dirs = list
	} else {
		var list fileInfoDirs
		list, err = f.Readdir(-1)
		dirs = list
	}

	if err != nil {
		logf(r, "http: error reading directory: %v", err)
		Error(w, "Error reading directory", http.StatusInternalServerError)
		return
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs.name(i) < dirs.name(j) })

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, "<pre>\n")
	for i, n := 0, dirs.len(); i < n; i++ {
		name := dirs.name(i)
		if dirs.isDir(i) {
			name += "/"
		}
		url := url.URL{Path: name}
		fmt.Fprintf(w, "<a href=\"%s\">%s</a>\n", url.String(), htmlReplacer.Replace(name))
	}
	fmt.Fprintf(w, "</pre>\n")
}

func ServeContent(w http.ResponseWriter, req *http.Request, name string, modtime time.Time, content io.ReadSeeker) {
	sizeFunc := func() (int64, error) {
		size, err := content.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, errSeeker
		}
		_, err = content.Seek(0, io.SeekStart)
		if err != nil {
			return 0, errSeeker
		}
		return size, nil
	}
	serveContent(w, req, name, modtime, sizeFunc, content)
}

var errSeeker = errors.New("seeker can't seek")
var errNoOverlap = errors.New("invalid range: failed to overlap")

func serveContent(w http.ResponseWriter, r *http.Request, name string, modtime time.Time, sizeFunc func() (int64, error), content io.ReadSeeker) {
	setLastModified(w, modtime)
	done, rangeReq := checkPreconditions(w, r, modtime)
	if done {
		return
	}

	code := http.StatusOK
	ctypes, haveType := w.Header()["Content-Type"]
	var ctype string
	if !haveType {
		ctype = mime.TypeByExtension(filepath.Ext(name))
		if ctype == "" {
			var buf [sniffLen]byte
			n, _ := io.ReadFull(content, buf[:])
			ctype = http.DetectContentType(buf[:n])
			_, err := content.Seek(0, io.SeekStart)
			if err != nil {
				Error(w, "seeker can't seek", http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", ctype)
	} else if len(ctypes) > 0 {
		ctype = ctypes[0]
	}

	size, err := sizeFunc()
	if err != nil {
		Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sendSize := size
	var sendContent io.Reader = content
	if size >= 0 {
		ranges, err := parseRange(rangeReq, size)
		if err != nil {
			if err == errNoOverlap {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
			}
			Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if sumRangesSize(ranges) > size {
			ranges = nil
		}
		switch {
		case len(ranges) == 1:
			ra := ranges[0]
			if _, err := content.Seek(ra.start, io.SeekStart); err != nil {
				Error(w, err.Error(), http.StatusRequestedRangeNotSatisfiable)
				return
			}
			sendSize = ra.length
			code = http.StatusPartialContent
			w.Header().Set("Content-Range", ra.contentRange(size))
		case len(ranges) > 1:
			sendSize = rangesMIMESize(ranges, ctype, size)
			code = http.StatusPartialContent
			pr, pw := io.Pipe()
			mw := multipart.NewWriter(pw)
			w.Header().Set("Content-Type", "multipart/byteranges; boundary="+mw.Boundary())
			sendContent = pr
			defer pr.Close()
			go func() {
				for _, ra := range ranges {
					part, err := mw.CreatePart(ra.mimeHeader(ctype, size))
					if err != nil {
						pw.CloseWithError(err)
						return
					}
					if _, err := content.Seek(ra.start, io.SeekStart); err != nil {
						pw.CloseWithError(err)
						return
					}
					if _, err := io.CopyN(part, content, ra.length); err != nil {
						pw.CloseWithError(err)
						return
					}
				}
				mw.Close()
				pw.Close()
			}()
		}

		w.Header().Set("Accept-Ranges", "bytes")
		if w.Header().Get("Content-Encoding") == "" {
			w.Header().Set("Content-Length", strconv.FormatInt(sendSize, 10))
		}
	}

	w.WriteHeader(code)
	if r.Method != "HEAD" {
		io.CopyN(w, sendContent, sendSize)
	}
}

func scanETag(s string) (etag string, remain string) {
	s = textproto.TrimString(s)
	start := 0
	if strings.HasPrefix(s, "W/") {
		start = 2
	}
	if len(s[start:]) < 2 || s[start] != '"' {
		return "", ""
	}
	for i := start + 1; i < len(s); i++ {
		c := s[i]
		switch {
		case c == 0x21 || c >= 0x23 && c <= 0x7E || c >= 0x80:
		case c == '"':
			return s[:i+1], s[i+1:]
		default:
			return "", ""
		}
	}
	return "", ""
}

func etagStrongMatch(a, b string) bool { return a == b && a != "" && a[0] == '"' }
func etagWeakMatch(a, b string) bool   { return strings.TrimPrefix(a, "W/") == strings.TrimPrefix(b, "W/") }

type condResult int
const ( condNone condResult = iota; condTrue; condFalse )

func checkIfMatch(w http.ResponseWriter, r *http.Request) condResult {
	im := r.Header.Get("If-Match")
	if im == "" { return condNone }
	for {
		im = textproto.TrimString(im)
		if len(im) == 0 { break }
		if im[0] == ',' { im = im[1:]; continue }
		if im[0] == '*' { return condTrue }
		etag, remain := scanETag(im)
		if etag == "" { break }
		if etagStrongMatch(etag, w.Header().Get("Etag")) { return condTrue }
		im = remain
	}
	return condFalse
}

func checkIfUnmodifiedSince(r *http.Request, modtime time.Time) condResult {
	ius := r.Header.Get("If-Unmodified-Since")
	if ius == "" || isZeroTime(modtime) { return condNone }
	t, err := http.ParseTime(ius)
	if err != nil { return condNone }
	modtime = modtime.Truncate(time.Second)
	if modtime.Before(t) || modtime.Equal(t) { return condTrue }
	return condFalse
}

func checkIfNoneMatch(w http.ResponseWriter, r *http.Request) condResult {
	inm := r.Header.Get("If-None-Match")
	if inm == "" { return condNone }
	buf := inm
	for {
		buf = textproto.TrimString(buf)
		if len(buf) == 0 { break }
		if buf[0] == ',' { buf = buf[1:]; continue }
		if buf[0] == '*' { return condFalse }
		etag, remain := scanETag(buf)
		if etag == "" { break }
		if etagWeakMatch(etag, w.Header().Get("Etag")) { return condFalse }
		buf = remain
	}
	return condTrue
}

func checkIfModifiedSince(r *http.Request, modtime time.Time) condResult {
	if r.Method != "GET" && r.Method != "HEAD" { return condNone }
	ims := r.Header.Get("If-Modified-Since")
	if ims == "" || isZeroTime(modtime) { return condNone }
	t, err := http.ParseTime(ims)
	if err != nil { return condNone }
	modtime = modtime.Truncate(time.Second)
	if modtime.Before(t) || modtime.Equal(t) { return condFalse }
	return condTrue
}

func checkIfRange(w http.ResponseWriter, r *http.Request, modtime time.Time) condResult {
	if r.Method != "GET" && r.Method != "HEAD" { return condNone }
	ir := r.Header.Get("If-Range")
	if ir == "" { return condNone }
	etag, _ := scanETag(ir)
	if etag != "" {
		if etagStrongMatch(etag, w.Header().Get("Etag")) { return condTrue
		} else { return condFalse }
	}
	if modtime.IsZero() { return condFalse }
	t, err := http.ParseTime(ir)
	if err != nil { return condFalse }
	if t.Unix() == modtime.Unix() { return condTrue }
	return condFalse
}

var unixEpochTime = time.Unix(0, 0)
func isZeroTime(t time.Time) bool { return t.IsZero() || t.Equal(unixEpochTime) }

func setLastModified(w http.ResponseWriter, modtime time.Time) {
	if !isZeroTime(modtime) {
		w.Header().Set("Last-Modified", modtime.UTC().Format(http.TimeFormat))
	}
}

func writeNotModified(w http.ResponseWriter) {
	h := w.Header()
	delete(h, "Content-Type")
	delete(h, "Content-Length")
	if h.Get("Etag") != "" { delete(h, "Last-Modified") }
	w.WriteHeader(http.StatusNotModified)
}

func checkPreconditions(w http.ResponseWriter, r *http.Request, modtime time.Time) (done bool, rangeHeader string) {
	ch := checkIfMatch(w, r)
	if ch == condNone { ch = checkIfUnmodifiedSince(r, modtime) }
	if ch == condFalse { w.WriteHeader(http.StatusPreconditionFailed); return true, "" }
	switch checkIfNoneMatch(w, r) {
	case condFalse:
		if r.Method == "GET" || r.Method == "HEAD" { writeNotModified(w); return true, ""
		} else { w.WriteHeader(http.StatusPreconditionFailed); return true, "" }
	case condNone:
		if checkIfModifiedSince(r, modtime) == condFalse { writeNotModified(w); return true, "" }
	}
	rangeHeader = r.Header.Get("Range")
	if rangeHeader != "" && checkIfRange(w, r, modtime) == condFalse { rangeHeader = "" }
	return false, rangeHeader
}

func serveFile(w http.ResponseWriter, r *http.Request, fsys http.FileSystem, name string, redirect bool) {
	const indexPage = "/index.html"
	if strings.HasSuffix(r.URL.Path, indexPage) { localRedirect(w, r, "./"); return }

	var f http.File
	var err error

	q := r.URL.Query()
	maxWidth_ := q.Get("max-width")
	if len(maxWidth_) > 0 {
		if maxWidth, errConv := strconv.Atoi(maxWidth_); errConv == nil {
			hasher := md5.New()
			hasher.Write([]byte(name))
			s := hex.EncodeToString(hasher.Sum(nil))
			ext := filepath.Ext(name)
			dirn := filepath.Join(tmpdir, s[0:1], s[1:3])
			os.MkdirAll(dirn, 0755)
			fn := filepath.Join(dirn, fmt.Sprintf("%s_%d%s", s[3:], maxWidth, ext))
			if f, err = os.Open(fn); err == nil {
				defer f.Close()
				goto GOT
			} else { log.Println(err) }

			if reader, errOpen := fsys.Open(name); errOpen == nil {
				defer reader.Close()
				if im, _, errDecode := image.DecodeConfig(reader); errDecode == nil {
					if im.Width > maxWidth {
						rImg, _ := fsys.Open(name); defer rImg.Close()
						if src, errImaging := imaging.Decode(rImg); errImaging == nil {
							dst := imaging.Resize(src, maxWidth, 0, imaging.Lanczos)
							if errSave := imaging.Save(dst, fn); errSave == nil {
								f, err = os.Open(fn)
								if err != nil { msg, code := toHTTPError(err); Error(w, msg, code); return }
								defer f.Close()
							} else { log.Println(errSave) }
						}
					} else {
						rImg, _ := fsys.Open(name); defer rImg.Close()
						dstFile, _ := os.Create(fn); defer dstFile.Close()
						io.Copy(dstFile, rImg)
						f, _ = os.Open(fn); goto GOT
					}
				}
			} else { log.Println(errOpen) }
		}
	}
GOT:
	if f == nil {
		f, err = fsys.Open(name)
		if err != nil { msg, code := toHTTPError(err); Error(w, msg, code); return }
		defer f.Close()
	}

	d, err := f.Stat()
	if err != nil { msg, code := toHTTPError(err); Error(w, msg, code); return }

	if redirect {
		url := r.URL.Path
		if d.IsDir() {
			if url[len(url)-1] != '/' { localRedirect(w, r, path.Base(url)+"/"); return }
		} else {
			if url[len(url)-1] == '/' { localRedirect(w, r, "../"+path.Base(url)); return }
		}
	}

	if d.IsDir() {
		url := r.URL.Path
		if url == "" || url[len(url)-1] != '/' { localRedirect(w, r, path.Base(url)+"/"); return }
		index := strings.TrimSuffix(name, "/") + indexPage
		ff, err := fsys.Open(index)
		if err == nil {
			defer ff.Close()
			dd, err := ff.Stat()
			if err == nil { name = index; d = dd; f = ff }
		}
	}

	if d.IsDir() {
		if checkIfModifiedSince(r, d.ModTime()) == condFalse { writeNotModified(w); return }
		setLastModified(w, d.ModTime())
		dirList(w, r, f); return
	}

	sizeFunc := func() (int64, error) { return d.Size(), nil }
	serveContent(w, r, d.Name(), d.ModTime(), sizeFunc, f)
}

func toHTTPError(err error) (msg string, httpStatus int) {
	if errors.Is(err, fs.ErrNotExist) { return "404 page not found", http.StatusNotFound }
	if errors.Is(err, fs.ErrPermission) { return "403 Forbidden", http.StatusForbidden }
	return "500 Internal Server Error", http.StatusInternalServerError
}

func localRedirect(w http.ResponseWriter, r *http.Request, newPath string) {
	if q := r.URL.RawQuery; q != "" { newPath += "?" + q }
	w.Header().Set("Location", newPath)
	w.WriteHeader(http.StatusMovedPermanently)
}

func ServeFile(w http.ResponseWriter, r *http.Request, name string) {
	if containsDotDot(r.URL.Path) { Error(w, "invalid URL path", http.StatusBadRequest); return }
	dir, file := filepath.Split(name)
	serveFile(w, r, Dir(dir), file, false)
}

func containsDotDot(v string) bool {
	if !strings.Contains(v, "..") { return false }
	for _, ent := range strings.FieldsFunc(v, isSlashRune) { if ent == ".." { return true } }
	return false
}
func isSlashRune(r rune) bool { return r == '/' || r == '\\' }

type fileHandler struct { root http.FileSystem }
type ioFS struct { fsys fs.FS }
type ioFile struct { file fs.File }

func (f ioFS) Open(name string) (http.File, error) {
	if name == "/" { name = "." } else { name = strings.TrimPrefix(name, "/") }
	file, err := f.fsys.Open(name)
	if err != nil { return nil, mapOpenError(err, name, '/', func(path string) (fs.FileInfo, error) { return fs.Stat(f.fsys, path) }) }
	return ioFile{file}, nil
}
func (f ioFile) Close() error               { return f.file.Close() }
func (f ioFile) Read(b []byte) (int, error) { return f.file.Read(b) }
func (f ioFile) Stat() (fs.FileInfo, error) { return f.file.Stat() }

var errMissingSeek = errors.New("io.File missing Seek method")
var errMissingReadDir = errors.New("io.File directory missing ReadDir method")

func (f ioFile) Seek(offset int64, whence int) (int64, error) {
	s, ok := f.file.(io.Seeker); if !ok { return 0, errMissingSeek }; return s.Seek(offset, whence)
}
func (f ioFile) ReadDir(count int) ([]fs.DirEntry, error) {
	d, ok := f.file.(fs.ReadDirFile); if !ok { return nil, errMissingReadDir }; return d.ReadDir(count)
}
func (f ioFile) Readdir(count int) ([]fs.FileInfo, error) {
	d, ok := f.file.(fs.ReadDirFile); if !ok { return nil, errMissingReadDir }
	var list []fs.FileInfo
	for {
		dirs, err := d.ReadDir(count - len(list))
		for _, dir := range dirs { info, err := dir.Info(); if err != nil { continue }; list = append(list, info) }
		if err != nil { return list, err }
		if count < 0 || len(list) >= count { break }
	}
	return list, nil
}
func FS(fsys fs.FS) http.FileSystem { return ioFS{fsys} }
func FileServer(root http.FileSystem) http.Handler { return &fileHandler{root} }
func (f *fileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	upath := r.URL.Path
	if !strings.HasPrefix(upath, "/") { upath = "/" + upath; r.URL.Path = upath }
	serveFile(w, r, f.root, path.Clean(upath), true)
}

type httpRange struct { start, length int64 }
func (r httpRange) contentRange(size int64) string { return fmt.Sprintf("bytes %d-%d/%d", r.start, r.start+r.length-1, size) }
func (r httpRange) mimeHeader(contentType string, size int64) textproto.MIMEHeader {
	return textproto.MIMEHeader{ "Content-Range": {r.contentRange(size)}, "Content-Type":  {contentType}, }
}
func parseRange(s string, size int64) ([]httpRange, error) {
	if s == "" { return nil, nil }
	const b = "bytes="
	if !strings.HasPrefix(s, b) { return nil, errors.New("invalid range") }
	var ranges []httpRange
	noOverlap := false
	for _, ra := range strings.Split(s[len(b):], ",") {
		ra = textproto.TrimString(ra)
		if ra == "" { continue }
		start, end, ok := strings.Cut(ra, "-")
		if !ok { return nil, errors.New("invalid range") }
		start, end = textproto.TrimString(start), textproto.TrimString(end)
		var r httpRange
		if start == "" {
			if end == "" || end[0] == '-' { return nil, errors.New("invalid range") }
			i, err := strconv.ParseInt(end, 10, 64)
			if i < 0 || err != nil { return nil, errors.New("invalid range") }
			if i > size { i = size }
			r.start = size - i; r.length = size - r.start
		} else {
			i, err := strconv.ParseInt(start, 10, 64)
			if err != nil || i < 0 { return nil, errors.New("invalid range") }
			if i >= size { noOverlap = true; continue }
			r.start = i
			if end == "" { r.length = size - r.start
			} else {
				i, err := strconv.ParseInt(end, 10, 64)
				if err != nil || r.start > i { return nil, errors.New("invalid range") }
				if i >= size { i = size - 1 }
				r.length = i - r.start + 1
			}
		}
		ranges = append(ranges, r)
	}
	if noOverlap && len(ranges) == 0 { return nil, errNoOverlap }
	return ranges, nil
}
type countingWriter int64
func (w *countingWriter) Write(p []byte) (n int, err error) { *w += countingWriter(len(p)); return len(p), nil }
func rangesMIMESize(ranges []httpRange, contentType string, contentSize int64) (encSize int64) {
	var w countingWriter; mw := multipart.NewWriter(&w)
	for _, ra := range ranges { mw.CreatePart(ra.mimeHeader(contentType, contentSize)); encSize += ra.length }
	mw.Close(); encSize += int64(w); return
}
func sumRangesSize(ranges []httpRange) (size int64) { for _, ra := range ranges { size += ra.length }; return }
