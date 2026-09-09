package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"go-music/internal/config"
)

var testFS = fstest.MapFS{
	"index.html": &fstest.MapFile{Data: []byte("<title>堆栈音乐播放器</title>")},
}

func testServer() *Server {
	return New(config.Default(), nil, nil, nil, testFS)
}

// TestStaticRoutes 验证根路径返回前端页面 HTML。
func TestStaticRoutes(t *testing.T) {
	s := testServer()
	r := s.buildRouter()

	cases := []struct {
		path string
		want int
	}{
		{"/", 200},
		// Go 1.25 起 FileServer 将 /index.html 规范化重定向（301 到 /），浏览器会自动跟随
		{"/index.html", 301},
	}
	for _, c := range cases {
		req := httptest.NewRequest(http.MethodGet, c.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != c.want {
			t.Errorf("%s 期望 %d，得到 %d (loc=%q)", c.path, c.want, w.Code, w.Header().Get("Location"))
			continue
		}
		if c.want != 200 {
			continue // 重定向响应无内容可检查
		}
		body, _ := io.ReadAll(w.Result().Body)
		if strings.Contains(string(body), "堆栈音乐播放器") == false {
			t.Errorf("%s 返回内容缺少 堆栈音乐播放器标识: %d bytes", c.path, len(body))
		}
	}
}

// TestAPIPathGuard 非 /api 的未知路径不应返回 404 JSON（由 FileServer 处理）。
func TestAPIPathGuard(t *testing.T) {
	s := testServer()
	r := s.buildRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/unknown-xyz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 404 {
		t.Errorf("期望 404，得到 %d", w.Code)
	}
}
