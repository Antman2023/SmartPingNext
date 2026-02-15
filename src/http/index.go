package http

import (
	"net/http"
	"path/filepath"
	"smartping/src/g"
	"strings"
)

func configIndexRoutes() {

	// 优先尝试 html 目录（Vue SPA 构建输出）
	htmlPath := filepath.Join(g.Root, "/html")
	indexPath := filepath.Join(htmlPath, "index.html")
	if g.IsExist(indexPath) {
		fs := http.FileServer(http.Dir(htmlPath))

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
				fs.ServeHTTP(w, r)
				return
			}

			// SPA 路由：所有其他路径返回 index.html
			http.ServeFile(w, r, indexPath)
		})
		return
	}

	// 回退到开发模式：web/dist 目录
	vueDistPath := filepath.Join(g.Root, "/web/dist")
	if g.IsExist(vueDistPath) {
		fs := http.FileServer(http.Dir(vueDistPath))

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
				fs.ServeHTTP(w, r)
				return
			}

			// SPA 路由：所有其他路径返回 index.html
			http.ServeFile(w, r, filepath.Join(vueDistPath, "index.html"))
		})
		return
	}

	// 没有找到前端文件
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Frontend files not found. Please build the web app first.", http.StatusNotFound)
	})

}
