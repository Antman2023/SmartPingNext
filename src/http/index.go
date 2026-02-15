package http

import (
	"net/http"
	"path/filepath"
	"smartping/src/g"
	"strings"
)

func configIndexRoutes() {

	// 优先尝试 Vue SPA 构建目录
	vueDistPath := filepath.Join(g.Root, "/web/dist")
	if g.IsExist(vueDistPath) {
		fs := http.FileServer(http.Dir(vueDistPath))

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			if !AuthUserIp(r.RemoteAddr) {
				o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
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

	// 回退到旧的 HTML 目录
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !AuthUserIp(r.RemoteAddr) {
			o := "Your ip address (" + r.RemoteAddr + ")  is not allowed to access this site!"
			http.Error(w, o, http.StatusUnauthorized)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/") {
			if !g.IsExist(filepath.Join(g.Root, "/html", r.URL.Path, "index.html")) {
				http.NotFound(w, r)
				return
			}
		}
		http.FileServer(http.Dir(filepath.Join(g.Root, "/html"))).ServeHTTP(w, r)
	})

}
