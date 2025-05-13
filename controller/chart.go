package controller

//浏览器发请求 → 检查缓存 → 请求 GitHub → 渲染图表 → 返回 → 缓存写入
import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/caarlos0/httperr"
	"github.com/caarlos0/starcharts/internal/cache"
	"github.com/caarlos0/starcharts/internal/chart"
	"github.com/caarlos0/starcharts/internal/chart/svg"
	"github.com/caarlos0/starcharts/internal/github"
)

var stylesMap = map[string]string{ //根据用户选择的图表返回chart里不同的图表主题
	"light":    chart.LightStyles,
	"dark":     chart.DarkStyles,
	"adaptive": chart.AdaptiveStyles,
}

// GetRepoChart returns the SVG chart for the given repository.
//
// nolint: funlen
// TODO: refactor.
func GetRepoChart(gh *github.GitHub, cache *cache.Redis) http.Handler {
	return httperr.NewF(func(w http.ResponseWriter, r *http.Request) error { //又return了一个函数
		params, err := extractSvgChartParams(r)
		if err != nil {
			log.WithError(err).Error("failed to extract params")
			return err
		}

		cacheKey := chartKey(params) //构建一个唯一缓存键 设置日志上下文
		name := fmt.Sprintf("%s/%s", params.Owner, params.Repo)
		log := log.WithField("repo", name).WithField("variant", params.Variant)

		cachedChart := "" //尝试从Redis缓存中读取图表，直接返回
		if err = cache.Get(cacheKey, &cachedChart); err == nil {
			writeSvgHeaders(w)
			log.Debug("using cached chart")
			_, err := fmt.Fprint(w, cachedChart)
			return err
		}

		defer log.Trace("collect_stars").Stop(nil)
		repo, err := gh.RepoDetails(r.Context(), name) //获取仓库数据
		if err != nil {
			return httperr.Wrap(err, http.StatusBadRequest)
		}

		stargazers, err := gh.Stargazers(r.Context(), repo) //获取Stargazer数据
		if err != nil {
			log.WithError(err).Error("failed to get stars")
			writeSvgHeaders(w)
			_, err = w.Write([]byte(errSvg(err)))
			return err
		}

		series := chart.Series{ //构建折现图数据
			StrokeWidth: 2,
			Color:       params.Line,
		}
		for i, star := range stargazers {
			series.XValues = append(series.XValues, star.StarredAt)
			series.YValues = append(series.YValues, float64(i+1))
		}
		if len(series.XValues) < 2 { //防止数据不足崩图
			log.Info("not enough results, adding some fake ones")
			series.XValues = append(series.XValues, time.Now())
			series.YValues = append(series.YValues, 1)
		}

		graph := &chart.Chart{ //构建图表对象
			Width:      CHART_WIDTH,
			Height:     CHART_HEIGHT,
			Styles:     stylesMap[params.Variant],
			Background: params.Background,
			XAxis: chart.XAxis{
				Name:        "Time",
				Color:       params.Axis,
				StrokeWidth: 2,
			},
			YAxis: chart.YAxis{
				Name:        "Stargazers",
				Color:       params.Axis,
				StrokeWidth: 2,
			},
			Series: series,
		}
		defer log.Trace("chart").Stop(&err)

		writeSvgHeaders(w) //设置响应头，渲染图表

		cacheBuffer := &strings.Builder{}
		graph.Render(io.MultiWriter(w, cacheBuffer))
		err = cache.Put(cacheKey, cacheBuffer.String()) //缓存图表数据
		if err != nil {
			log.WithError(err).Error("failed to cache chart")
		}

		return nil
	})
}

func errSvg(err error) string { //错误处理
	return svg.SVG().
		Attr("width", svg.Px(CHART_WIDTH)).
		Attr("height", svg.Px(CHART_HEIGHT)).
		ContentFunc(func(writer io.Writer) {
			svg.Text().
				Attr("fill", "red").
				Attr("x", svg.Px(CHART_WIDTH/2)).
				Attr("y", svg.Px(CHART_HEIGHT/2)).
				Content(err.Error()).
				Render(writer)
		}).
		String()
}
