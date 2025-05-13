package controller

import (
	"fmt"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/caarlos0/httperr"
	"github.com/caarlos0/starcharts/internal/cache"
	"github.com/caarlos0/starcharts/internal/github"
	"github.com/gorilla/mux"
)

const (
	CHART_WIDTH  = 1024
	CHART_HEIGHT = 400
)

// GetRepo shows the given repo chart.
func GetRepo(fsys fs.FS, gh *github.GitHub, cache *cache.Redis, version string) http.Handler {
	repositoryTemplate, err := template.ParseFS(fsys, base, repository) //加载模板文件
	if err != nil {
		panic(err)
	}

	indexTemplate, err := template.ParseFS(fsys, base, index)
	if err != nil {
		panic(err)
	}

	return httperr.NewF(func(w http.ResponseWriter, r *http.Request) error { //解析请求中的Github仓库名
		name := fmt.Sprintf(
			"%s/%s",
			mux.Vars(r)["owner"],
			mux.Vars(r)["repo"],
		)
		details, err := gh.RepoDetails(r.Context(), name) //调用Github API获取仓库信息
		if err != nil {
			return indexTemplate.Execute(w, map[string]error{
				"Error": err,
			})
		}

		return repositoryTemplate.Execute(w, map[string]interface{}{
			"Version": version,
			"Details": details,
		})
	})
}
