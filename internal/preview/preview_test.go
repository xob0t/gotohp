package preview

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func newTestHandler(s *Server) http.Handler {
	return s.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
}

func get(t *testing.T, h http.Handler, method, target string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for key, values := range header {
		req.Header[key] = values
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func previewURL(path string) string {
	return Route + "?path=" + url.QueryEscape(path)
}

func TestMiddlewareServesOnlyAllowedMedia(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "IMG 0001.JPG")
	other := filepath.Join(dir, "other.jpg")
	secret := filepath.Join(dir, "notes.txt")
	for _, path := range []string{photo, other, secret} {
		if err := os.WriteFile(path, []byte("0123456789"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	s := New()
	s.Allow(photo)
	s.Allow(secret)                         // not a media extension, must be ignored
	s.Allow(filepath.Join(dir, "gone.jpg")) // reported, but no longer on disk
	s.Allow(dir + string(filepath.Separator) + "folder.jpg")
	if err := os.Mkdir(filepath.Join(dir, "folder.jpg"), 0o700); err != nil {
		t.Fatal(err)
	}
	h := newTestHandler(s)

	if rec := get(t, h, http.MethodGet, previewURL(photo), nil); rec.Code != http.StatusOK || rec.Body.String() != "0123456789" {
		t.Fatalf("allowed photo: got %d %q", rec.Code, rec.Body.String())
	}
	if rec := get(t, h, http.MethodGet, previewURL(photo), http.Header{"Range": {"bytes=2-4"}}); rec.Code != http.StatusPartialContent || rec.Body.String() != "234" {
		t.Fatalf("range request: got %d %q", rec.Code, rec.Body.String())
	}
	for name, target := range map[string]string{
		"file never reported":   previewURL(other),
		"non-media file":        previewURL(secret),
		"directory":             previewURL(filepath.Join(dir, "folder.jpg")),
		"missing path":          Route,
		"path traversal":        previewURL(dir + "/../" + filepath.Base(dir) + "/IMG 0001.JPG"),
		"allowed, then removed": previewURL(filepath.Join(dir, "gone.jpg")),
	} {
		if rec := get(t, h, http.MethodGet, target, nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: got %d, want 404", name, rec.Code)
		}
	}
	if rec := get(t, h, http.MethodPost, previewURL(photo), nil); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rec.Code)
	}
	if rec := get(t, h, http.MethodGet, "/index.html", nil); rec.Code != http.StatusTeapot {
		t.Errorf("other routes must reach the next handler, got %d", rec.Code)
	}
}

func TestAllowStartsOverWhenFull(t *testing.T) {
	s := New()
	for i := range maxAllowed {
		s.Allow("photo-" + strconv.Itoa(i) + ".jpg")
	}
	if len(s.allowed) != maxAllowed {
		t.Fatalf("got %d entries, want %d", len(s.allowed), maxAllowed)
	}
	s.Allow("fresh.jpg")
	if len(s.allowed) != 1 || !s.isAllowed("fresh.jpg") {
		t.Fatalf("set should restart with the new path, has %d entries", len(s.allowed))
	}
}
