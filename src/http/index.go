package http

import (
	"net/http"
	"path/filepath"
	"smartping/src/g"
	"strings"
)

func configIndexRoutes() {

	// 尝试查找前端文件目录
	// 优先级：html/ > web/dist/
	var staticPath string
	var indexPath string

	htmlPath := filepath.Join(g.Root, "/html")
	htmlIndex := filepath.Join(htmlPath, "index.html")
	distPath := filepath.Join(g.Root, "/web/dist")
	distIndex := filepath.Join(distPath, "index.html")

	if g.IsExist(htmlIndex) {
		staticPath = htmlPath
		indexPath = htmlIndex
	} else if g.IsExist(distIndex) {
		staticPath = distPath
		indexPath = distIndex
	}

	if staticPath != "" {
		fs := http.FileServer(http.Dir(staticPath))

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

	// 没有找到前端文件
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Frontend files not found. Please run: cd web && npm run build", http.StatusNotFound)
	})

}
