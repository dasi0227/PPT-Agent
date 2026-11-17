package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// withFrontend 在同一地址提供后端 API 和构建后的前端，供桌面版等场景使用。
// 前端页面路由回退到 index.html，刷新或直接打开页面地址时仍能加载应用。
func withFrontend(api http.Handler, directory string) (http.Handler, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	index := filepath.Join(root, "index.html")
	if info, err := os.Stat(index); err != nil || info.IsDir() {
		return nil, fmt.Errorf("frontend index.html not found in %s", root)
	}
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API（包括 SSE）仍交给原路由处理，不能落入前端页面的回退逻辑。
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// 优先返回实际存在的 HTML、JS、CSS、图片等静态文件。
		clean := path.Clean("/" + r.URL.Path)
		file := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(clean, "/")))
		if info, err := os.Stat(file); err == nil {
			if info.IsDir() {
				info, err = os.Stat(filepath.Join(file, "index.html"))
			}
			if err == nil && !info.IsDir() {
				w.Header().Set("Cache-Control", "no-cache")
				files.ServeHTTP(w, r)
				return
			}
		}
		// 仅将请求 HTML 且不带文件扩展名的路径视为页面路由；缺失的静态资源返回 404。
		if path.Ext(clean) == "" && strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, index)
			return
		}
		http.NotFound(w, r)
	}), nil
}
