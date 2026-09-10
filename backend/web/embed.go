package web

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// Handler 返回前端静态文件服务。
func Handler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

// SpaHandler 服务前端静态文件；非 /api 且不存在的路径回退到 index.html。
func SpaHandler() http.Handler {
	fileServer := Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}
		// 尝试打开文件；不存在则回退 index.html（SPA 路由）
		if _, err := fs.Stat(distFS, "dist"+strings.TrimPrefix(p, "/")); err != nil {
			b, err := distFS.ReadFile("dist/index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(b)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
