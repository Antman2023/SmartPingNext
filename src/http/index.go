package http

import (
	"io/fs"
	"net/http"
	"smartping/src/static"
	"strings"
)

func configIndexRoutes() {

	// 使用嵌入的前端文件系统
	htmlFS, err := fs.Sub(static.HTML, "html")
	if err != nil {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "Failed to load embedded files: "+err.Error(), http.StatusInternalServerError)
		})
		return
	}

	fileServer := http.FileServer(http.FS(htmlFS))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) {
			o := "Your ip address (" + r.RemoteAddr + ") is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}

		// API 请求交给其他路由处理
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// 静态资源文件（有扩展名）直接服务
		if strings.Contains(r.URL.Path, ".") {
			fileServer.ServeHTTP(w, r)
			return
		}

		// SPA 路由：所有其他路径返回 index.html
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})

}
