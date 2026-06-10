package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPagePreviewServerServesHTML(t *testing.T) {
	root := t.TempDir()
	htmlPath := filepath.Join(root, "index.html")
	if err := os.WriteFile(htmlPath, []byte("<html><body>hello</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := newPagePreviewServer()
	url, err := srv.previewURL(root, "index.html")
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, strings.TrimPrefix(url, "http://127.0.0.1:"+itoa(srv.port)), nil)
	if req.URL.Host == "" {
		req.URL.Path = "/p/" + tokenForBase(root) + "/index.html"
	}
	rec := httptest.NewRecorder()
	srv.serveHTTP(rec, httptest.NewRequest(http.MethodGet, "/p/"+tokenForBase(root)+"/index.html", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "hello") {
		t.Fatalf("body = %q", body)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
