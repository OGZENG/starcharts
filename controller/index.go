package controller

import (
	"html/template"
	"io/fs"
	"net/http"
	"strings"

	"github.com/caarlos0/httperr"
)

func Index(filesystem fs.FS, version string) http.Handler { /*第一个参数是嵌入的文件系统，用于读取模板文件，
	第二个是传入程序版本号*/
	indexTemplate, err := template.ParseFS(filesystem, base, index) //加载base.html和index.html两个模板文件并生成一个indexTemplate
	if err != nil {
		panic(err)
	}

	return httperr.NewF(func(w http.ResponseWriter, r *http.Request) error { //返回一个http处理器
		return indexTemplate.Execute(w, map[string]string{"Version": version})
	})
}

func HandleForm() http.HandlerFunc { //返回一个处理HTTP表单的函数
	return func(w http.ResponseWriter, r *http.Request) { //提取用户输入的Github仓库链接
		repo := strings.TrimPrefix(r.FormValue("repository"), "https://github.com/") //只保留用户名/仓库名
		http.Redirect(w, r, repo, http.StatusSeeOther)                               //重定向浏览器去访问这个仓库对应的路径页面
	}
}
