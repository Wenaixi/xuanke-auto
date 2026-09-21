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
		// /api 无尾斜杠时顶层 `/api` 前缀不匹配 router.go 的
		// mux.HandleFunc("/api/",...)，会落到本 SPA 兜底把 index.html 当 200 返回——
		// 未知 API 端点被安全扫描误判"任意路径可 200"。统一在这里
		// 把所有 /api 前缀（含精确 /api）拒为 404，与 router.go 的 /api/ 显式 404 同案。
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			http.NotFound(w, r)
			return
		}
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
