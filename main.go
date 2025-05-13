package main

import (
	"embed"    //将静态文件编译时嵌入到Go里
	"net/http" //构建http客户端和服务器功能
	"os"
	"time"

	"github.com/apex/httplog"
	"github.com/apex/log"
	"github.com/apex/log/handlers/text"
	"github.com/caarlos0/starcharts/config"
	"github.com/caarlos0/starcharts/controller"
	"github.com/caarlos0/starcharts/internal/cache"
	"github.com/caarlos0/starcharts/internal/github"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

//go:embed static/*
var static embed.FS //编译期间将文件或整个目录打包进Go程序里 file system

var version = "devel" //development 开发中的版本

func main() {
	log.SetHandler(text.New(os.Stderr)) //日志处理器 纯文本格式的日志 日志就是程序运行时发生的事情的记录
	// log.SetLevel(log.DebugLevel)
	config := config.Get()                          //从项目自定义的config包中获取配置
	ctx := log.WithField("listen", config.Listen)   //设置一个带上下文信息的日志对象
	options, err := redis.ParseURL(config.RedisURL) //解析Redis URL返回连接参数
	if err != nil {
		log.WithError(err).Fatal("invalid redis_url")
	}
	redis := redis.NewClient(options) /*创建一个新的Redis客户端实例 Redis就是一个运行在内存中的超快的数据库，
	类似于费大厨回锅肉这道菜会专门放出来*/
	cache := cache.New(redis)           //通过redis客户端创建一个缓存系统
	defer cache.Close()                 //才学的defer函数，当main结束后延迟执行close
	github := github.New(config, cache) //创建一个Github客户端或服务

	r := mux.NewRouter() //创建一个新的路由器对象r，用它来注册所有URL和它们对应的处理函数
	r.Path("/").         //访问主页 get，处理方式是调用controller的index函数
				Methods(http.MethodGet).
				Handler(controller.Index(static, version))
	r.Path("/").
		Methods(http.MethodPost). //提交表单时浏览器会发起一个post请求，由HandleForm处理
		HandlerFunc(controller.HandleForm())
	r.PathPrefix("/static/").
		Methods(http.MethodGet).                  //当用户请求资源时会从嵌入的static目录中读取文件
		Handler(http.FileServer(http.FS(static))) //用http.FileServer把嵌入的文件作为HTTP服务器的静态文件目录
	r.Path("/{owner}/{repo}.svg"). //返回SVG格式图表
					Methods(http.MethodGet).
					Handler(controller.GetRepoChart(github, cache))
	r.Path("/{owner}/{repo}"). //请求仓库信息页面
					Methods(http.MethodGet).
					Handler(controller.GetRepo(static, github, cache, version))
	//定义谁来处理什么
	// generic metrics
	requestCounter := promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "starcharts",
		Subsystem: "http",
		Name:      "requests_total",
		Help:      "total requests",
	}, []string{"code", "method"})
	responseObserver := promauto.NewSummaryVec(prometheus.SummaryOpts{
		Namespace: "starcharts",
		Subsystem: "http",
		Name:      "responses",
		Help:      "response times and counts",
	}, []string{"code", "method"})

	r.Methods(http.MethodGet).Path("/metrics").Handler(promhttp.Handler())

	srv := &http.Server{
		Handler: httplog.New(
			promhttp.InstrumentHandlerDuration(
				responseObserver,
				promhttp.InstrumentHandlerCounter(
					requestCounter,
					r,
				),
			),
		),
		Addr:         config.Listen,
		WriteTimeout: 60 * time.Second,
		ReadTimeout:  60 * time.Second,
	}
	ctx.Info("starting up...")
	ctx.WithError(srv.ListenAndServe()).Error("failed to start up server")
}
