package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

// newGzipRouter builds a router with the middleware in place and a single GET
// handler, mirroring how SetupRouter registers it.
func newGzipRouter(handler gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.Use(Gzip())
	r.GET("/", handler)
	return r
}

func doGet(r *gin.Engine, acceptEncoding string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if acceptEncoding != "" {
		req.Header.Set("Accept-Encoding", acceptEncoding)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGzipCompressesHTMLAndRoundTrips(t *testing.T) {
	// Repetitive, like the catalogue JSON the /songs page inlines.
	body := strings.Repeat("<div class=\"song-card\">Hurray Hurray LOVE</div>", 500)

	r := newGzipRouter(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(body))
	})
	w := doGet(r, "gzip")

	if got := w.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want %q", got, "gzip")
	}
	if got := w.Header().Get("Vary"); got != "Accept-Encoding" {
		t.Errorf("Vary = %q, want %q", got, "Accept-Encoding")
	}
	// A stale uncompressed length would make the client truncate the body.
	if got := w.Header().Get("Content-Length"); got != "" {
		t.Errorf("Content-Length = %q, want it removed", got)
	}

	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("response is not valid gzip: %v", err)
	}
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("reading gzip stream: %v", err)
	}
	if err := zr.Close(); err != nil {
		t.Fatalf("closing gzip stream: %v", err)
	}
	if string(got) != body {
		t.Fatalf("decompressed body does not match: got %d bytes, want %d", len(got), len(body))
	}
	if w.Body.Len() >= len(body) {
		t.Errorf("compressed size %d did not beat original %d", w.Body.Len(), len(body))
	}
}

func TestGzipPassesThroughWhenNotAccepted(t *testing.T) {
	body := strings.Repeat("hello world ", 500)
	r := newGzipRouter(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(body))
	})

	for _, accept := range []string{"", "identity", "br", "gzip;q=0"} {
		w := doGet(r, accept)
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("Accept-Encoding %q: Content-Encoding = %q, want none", accept, got)
		}
		if w.Body.String() != body {
			t.Errorf("Accept-Encoding %q: body was altered", accept)
		}
	}
}

func TestGzipSkipsBinaryContentTypes(t *testing.T) {
	for _, contentType := range []string{"image/png", "font/otf", "application/octet-stream", "video/mp4"} {
		r := newGzipRouter(func(c *gin.Context) {
			c.Data(http.StatusOK, contentType, bytes.Repeat([]byte{0xAB}, 4096))
		})
		if got := doGet(r, "gzip").Header().Get("Content-Encoding"); got != "" {
			t.Errorf("%s: Content-Encoding = %q, want none", contentType, got)
		}
	}
}

func TestGzipCompressesJSONAndJS(t *testing.T) {
	for _, contentType := range []string{"application/json; charset=utf-8", "text/javascript", "application/javascript", "text/css", "image/svg+xml"} {
		r := newGzipRouter(func(c *gin.Context) {
			c.Data(http.StatusOK, contentType, []byte(strings.Repeat("a", 4096)))
		})
		if got := doGet(r, "gzip").Header().Get("Content-Encoding"); got != "gzip" {
			t.Errorf("%s: Content-Encoding = %q, want gzip", contentType, got)
		}
	}
}

func TestGzipLeavesBodylessResponsesAlone(t *testing.T) {
	// 304s come from the static file server on every revalidation; tagging them
	// with an encoding they do not carry would be a lie.
	for _, status := range []int{http.StatusNoContent, http.StatusNotModified} {
		r := newGzipRouter(func(c *gin.Context) {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.Status(status)
		})
		w := doGet(r, "gzip")
		if got := w.Header().Get("Content-Encoding"); got != "" {
			t.Errorf("status %d: Content-Encoding = %q, want none", status, got)
		}
		if w.Body.Len() != 0 {
			t.Errorf("status %d: got a body of %d bytes", status, w.Body.Len())
		}
	}
}

func TestGzipSkipsWebSocketUpgrades(t *testing.T) {
	// The room routes hijack the connection; wrapping the writer must not
	// interfere with the handshake response.
	r := gin.New()
	r.Use(Gzip())
	r.GET("/ws", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html", []byte(strings.Repeat("x", 4096)))
	})

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if got := w.Header().Get("Content-Encoding"); got != "" {
		t.Errorf("Content-Encoding = %q, want none on an upgrade request", got)
	}
}

func TestGzipHandlesStreamedWrites(t *testing.T) {
	// c.String/WriteString takes a different path through the writer than
	// c.Data, and flushing mid-stream must not truncate the tail.
	r := newGzipRouter(func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; charset=utf-8")
		for i := 0; i < 20; i++ {
			c.Writer.WriteString("chunk\n")
			c.Writer.Flush()
		}
	})
	w := doGet(r, "gzip")

	zr, err := gzip.NewReader(w.Body)
	if err != nil {
		t.Fatalf("response is not valid gzip: %v", err)
	}
	got, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("reading gzip stream: %v", err)
	}
	if want := strings.Repeat("chunk\n", 20); string(got) != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// The writers are pooled across requests; a leaked reference would bleed one
// response into another.
func TestGzipPooledWritersStayIsolated(t *testing.T) {
	r := newGzipRouter(func(c *gin.Context) {
		c.Data(http.StatusOK, "text/plain", []byte(c.Query("body")))
	})

	for _, want := range []string{"first response", "second response", "third"} {
		req := httptest.NewRequest(http.MethodGet, "/?body="+url.QueryEscape(want), nil)
		req.Header.Set("Accept-Encoding", "gzip")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		zr, err := gzip.NewReader(w.Body)
		if err != nil {
			t.Fatalf("%q: not valid gzip: %v", want, err)
		}
		got, err := io.ReadAll(zr)
		if err != nil {
			t.Fatalf("%q: reading: %v", want, err)
		}
		if string(got) != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}
}
