package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// Gzip compresses text responses when the client advertises support.
//
// The pages are template-rendered HTML carrying the whole catalogue as inline
// JSON, which is extremely repetitive and compresses by roughly 90%. Neither
// gin nor the Traefik front end was compressing anything, so /songs was going
// over the wire at its full uncompressed size.
//
// Requests that are not plain responses are passed straight through: WebSocket
// upgrades (the room routes hijack the connection) and clients that did not ask
// for gzip.
func Gzip() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Caches must key on the encoding whether or not this particular
		// response ends up compressed.
		c.Writer.Header().Add("Vary", "Accept-Encoding")

		if !acceptsGzip(c.Request) || isWebSocketUpgrade(c.Request) {
			c.Next()
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: c.Writer}
		c.Writer = gw
		defer func() {
			gw.finish()
			c.Writer = gw.ResponseWriter
		}()

		c.Next()
	}
}

func acceptsGzip(r *http.Request) bool {
	for _, encoding := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		// Strip any q-value; a client that sends "gzip;q=0" is handled by the
		// explicit zero check below.
		name, params, _ := strings.Cut(encoding, ";")
		if strings.TrimSpace(name) != "gzip" {
			continue
		}
		return strings.TrimSpace(params) != "q=0"
	}
	return false
}

func isWebSocketUpgrade(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Upgrade"), "websocket") ||
		r.Header.Get("Sec-WebSocket-Key") != ""
}

// shouldCompress reports whether a Content-Type is worth compressing. Media and
// unknown/sniffed types are left alone: they are usually already compressed, and
// re-encoding them only burns CPU.
func shouldCompress(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.TrimSpace(strings.ToLower(mediaType))

	if strings.HasPrefix(mediaType, "text/") {
		return true
	}
	switch mediaType {
	case "application/json", "application/javascript", "application/xml",
		"application/rss+xml", "application/atom+xml", "image/svg+xml":
		return true
	}
	return false
}

var gzipWriterPool = sync.Pool{
	New: func() any {
		return gzip.NewWriter(io.Discard)
	},
}

// gzipResponseWriter wraps gin's writer and encodes the body on the way out.
// It embeds gin.ResponseWriter so Hijack, CloseNotify and friends still reach
// the real writer untouched.
type gzipResponseWriter struct {
	gin.ResponseWriter
	gz      *gzip.Writer
	decided bool
}

// prepare settles once, before the first byte reaches the client, whether this
// response gets compressed. It has to run from every entry point that can flush
// the header, because gin's WriteHeader only records the status and the actual
// header write happens later in WriteHeaderNow.
func (w *gzipResponseWriter) prepare() {
	if w.decided {
		return
	}
	w.decided = true

	// 204 and 304 carry no body, so an encoding header would only mislead.
	if status := w.Status(); status == http.StatusNoContent || status == http.StatusNotModified {
		return
	}
	if w.Written() || !shouldCompress(w.Header().Get("Content-Type")) {
		return
	}

	w.Header().Set("Content-Encoding", "gzip")
	// The handler's length describes the uncompressed body.
	w.Header().Del("Content-Length")

	w.gz = gzipWriterPool.Get().(*gzip.Writer)
	w.gz.Reset(w.ResponseWriter)
}

func (w *gzipResponseWriter) WriteHeaderNow() {
	w.prepare()
	w.ResponseWriter.WriteHeaderNow()
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	w.prepare()
	if w.gz == nil {
		return w.ResponseWriter.Write(data)
	}
	w.WriteHeaderNow()
	return w.gz.Write(data)
}

func (w *gzipResponseWriter) WriteString(s string) (int, error) {
	w.prepare()
	if w.gz == nil {
		return w.ResponseWriter.WriteString(s)
	}
	w.WriteHeaderNow()
	return w.gz.Write([]byte(s))
}

func (w *gzipResponseWriter) Flush() {
	if w.gz != nil {
		w.gz.Flush()
	}
	w.ResponseWriter.Flush()
}

// finish closes out the gzip stream and returns the writer to the pool. It must
// run after the handler chain, otherwise the trailing block is never emitted and
// the client sees a truncated body.
func (w *gzipResponseWriter) finish() {
	if w.gz == nil {
		return
	}
	w.gz.Close()
	// Drop the reference to this request's connection before pooling.
	w.gz.Reset(io.Discard)
	gzipWriterPool.Put(w.gz)
	w.gz = nil
}
