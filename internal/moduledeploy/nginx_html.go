package moduledeploy

import (
	"path/filepath"

	"github.com/wpg/wpgctl/internal/fetch"
	"github.com/wpg/wpgctl/internal/util"
)

// NginxLayout middleware/nginx 目录约定。
type NginxLayout struct {
	ModuleDir string // .../middleware/nginx
	HTMLDir   string // .../nginx/html
	ConfDir   string // .../nginx/conf/conf.d
	WebConf   string // .../http-web-8877.conf
}

// ResolveNginxLayout 解析 nginx 模块目录。
func ResolveNginxLayout(nginxModuleDir string) NginxLayout {
	root := filepath.Clean(nginxModuleDir)
	return NginxLayout{
		ModuleDir: root,
		HTMLDir:   filepath.Join(root, "html"),
		ConfDir:   filepath.Join(root, "conf", "conf.d"),
		WebConf:   filepath.Join(root, "conf", "conf.d", "http-web-8877.conf"),
	}
}

// ExpandNginxHTML 解压 html 目录下全部 .zip（可选；无 zip 时不报错）。
func ExpandNginxHTML(htmlDir string) (*fetch.ExpandResult, error) {
	if !util.DirExists(htmlDir) {
		return &fetch.ExpandResult{RootDir: htmlDir}, nil
	}
	return fetch.ExpandArchivesOptional(htmlDir)
}
