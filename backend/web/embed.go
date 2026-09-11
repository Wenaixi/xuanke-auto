package web

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed all:dist
var distFS embed.FS

// SpaHandler 服务前端静态文件；非 /api 且不存在的路径回退到 index.html。
func SpaHandler() http.Handler {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := path.Clean(r.URL.Path)
		if p == "/" || p == "." {
			fileServer.ServeHTTP(w, r)
			return
		}
		// 检查静态文件在嵌入式文件系统中是否存在
		trimmed := strings.TrimPrefix(p, "/")
		if _, err := fs.Stat(sub, trimmed); err == nil {
			// 文件存在，由标准 FileServer 负责响应，自动保证正确的 Content-Type 与缓存协商
			fileServer.ServeHTTP(w, r)
			return
		}
		// 文件不存在，SPA 单页应用回退到 index.html
		indexBytes, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(indexBytes)
	})
}
