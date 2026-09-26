package httpapi

import (
	"context"
	"strings"

	"github.com/dasi0227/PPT-Agent/backend/internal/fileopen"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLocalFileRequestRejectsCrossSiteNativeActions(t *testing.T) {
	for _, tc := range []struct {
		name, host, origin, content, site string
		allowed                           bool
	}{
		{"local", "localhost:5173", "http://localhost:5173", "application/json", "same-origin", true},
		{"loopback", "127.0.0.1:8787", "", "application/json", "", true},
		{"foreign origin", "localhost:8787", "https://evil.test", "application/json", "", false},
		{"cross site", "localhost:8787", "", "application/json", "cross-site", false},
		{"form", "localhost:8787", "", "application/x-www-form-urlencoded", "", false},
		{"dns rebinding", "evil.test:8787", "http://evil.test:8787", "application/json", "same-origin", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "http://"+tc.host+"/api/v1/files/open", nil)
			c.Request.Header.Set("Origin", tc.origin)
			c.Request.Header.Set("Content-Type", tc.content)
			c.Request.Header.Set("Sec-Fetch-Site", tc.site)
			if localFileRequest(c) != tc.allowed {
				t.Fatalf("status=%d", w.Code)
			}
		})
	}
}

type fileSettingsMemory struct{ value fileopen.Settings }

func (s *fileSettingsMemory) ReadFileSettings(context.Context) (fileopen.Settings, error) {
	return s.value, nil
}
func (s *fileSettingsMemory) WriteFileSettings(_ context.Context, edit fileopen.Settings) error {
	if edit.Revision != s.value.Revision {
		return fileopen.ErrConflict
	}
	edit.Revision++
	s.value = edit
	return nil
}
func TestFileSettingsHTTPContract(t *testing.T) {
	store := &fileSettingsMemory{value: fileopen.Settings{OpenWith: "system"}}
	router := &Router{engine: gin.New()}
	router.WithFileSettings(fileopen.NewService(store, t.TempDir()))
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, "http://localhost:8787"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.engine.ServeHTTP(w, req)
		return w
	}
	initial := request("GET", "/api/v1/settings/files", "")
	if initial.Code != 200 || !strings.Contains(initial.Body.String(), `"open_with":"system"`) {
		t.Fatal(initial.Body.String())
	}
	saved := request("PUT", "/api/v1/settings/files", `{"open_with":"finder","custom_app_path":"","revision":0}`)
	if saved.Code != 200 || !strings.Contains(saved.Body.String(), `"revision":1`) {
		t.Fatal(saved.Body.String())
	}
	stale := request("PUT", "/api/v1/settings/files", `{"open_with":"vscode","custom_app_path":"","revision":0}`)
	if stale.Code != 409 || store.value.OpenWith != "finder" {
		t.Fatal(stale.Body.String())
	}
	invalid := request("PUT", "/api/v1/settings/files", `{"open_with":"system","custom_app_path":"","revision":1,"unknown":true}`)
	if invalid.Code != 400 {
		t.Fatal(invalid.Body.String())
	}
	// A normal browser link must never invoke the native opener or picker.
	for _, path := range []string{"/api/v1/files/open?path=/tmp/slide.html", "/api/v1/settings/files/pick-application"} {
		if response := request("GET", path, ""); response.Code != 404 {
			t.Fatalf("GET %s: %d", path, response.Code)
		}
	}
}
