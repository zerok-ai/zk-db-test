package main

import (
	"fmt"
	"github.com/kataras/iris/v12"
	"github.com/prometheus/client_golang/prometheus"
	zkConfig "github.com/zerok-ai/zk-utils-go/config"
	zkHttpConfig "github.com/zerok-ai/zk-utils-go/http/config"
	zkLogger "github.com/zerok-ai/zk-utils-go/logs"
	"os"
	"redis-test/config"
	"redis-test/handlers"
	"redis-test/internal/common"
	"redis-test/internal/k8s"
	loadGenerators "redis-test/internal/load-generators"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	LogTag = "main"

	RedisWriteRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zk_redis_writes_request",
			Help: "Total number of objects to be written to redis",
		},
		[]string{"method"},
	)

	BadgerWriteRequestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zk_badger_writes_request",
			Help: "Total number of objects to be written to badger",
		},
		[]string{"method"},
	)
)

const (
	redisLoadTestApi         = "/gen-redis-load"
	badgerLoadTestApi        = "/gen-badger-load"
	badgerGetData            = "/get-badger-data"
	badgerGetTotalCount      = "/get-badger-total-count"
	badgerStartCompaction    = "/badger-start-compaction"
	badgerGetRandomData      = "/get-badger-random-data"
	redisLoadTestApiAllPods  = "/gen-redis-load-all"
	badgerLoadTestApiAllPods = "/gen-badger-load-all"

	namespace     = "zk-loadtest"
	serviceName   = "zk-db-test"
	labelSelector = "app=zk-db-test"
	port          = 80
)

func init() {
	prometheus.MustRegister(RedisWriteRequestCounter)
}

func main() {
	// read configuration from the file and environment variables
	var cfg config.AppConfigs
	if err := zkConfig.ProcessArgs[config.AppConfigs](&cfg); err != nil {
		panic(err)
	}

	buildNumber := os.Getenv("BUILD_NUMBER")
	zkLogger.Info(LogTag, "buildNumber:", buildNumber)
	zkLogger.Info(LogTag, "********* Initializing Application *********")
	zkHttpConfig.Init(cfg.Http.Debug)
	zkLogger.Init(cfg.LogsConfig)

	traceHandler, err := handlers.NewTraceHandler(&cfg)
	if err != nil {
		panic(err)
	}

	badgerLoadGenerator, err := loadGenerators.NewBadgerLoadGenerator(cfg, traceHandler)
	if err != nil {
		panic(err)
	}

	configurator := iris.WithConfiguration(iris.Configuration{
		DisablePathCorrection: true,
		LogLevel:              cfg.LogsConfig.Level,
	})
	if err = newApp(cfg, badgerLoadGenerator).Listen(":"+cfg.Server.Port, configurator); err != nil {
		panic(err)
	}

}

func newApp(cfg config.AppConfigs, badgerLoadGenerator *loadGenerators.BadgerLoadGenerator) *iris.Application {
	app := iris.Default()

	crs := func(ctx iris.Context) {
		ctx.Header("Access-Control-Allow-Credentials", "true")

		if ctx.Method() == iris.MethodOptions {
			ctx.Header("Access-Control-Methods",
				"POST, PUT, PATCH, DELETE")

			ctx.Header("Access-Control-Allow-Headers",
				"Access-Control-Allow-Origin,Content-Type")

			ctx.Header("Access-Control-Max-Age",
				"86400")

			ctx.StatusCode(iris.StatusNoContent)
			return
		}

		ctx.Next()
	}

	app.UseRouter(crs)
	app.AllowMethods(iris.MethodOptions)

	// Prometheus metric endpoint.
	app.Get("/metrics", iris.FromStd(promhttp.Handler()))

	// add other apis
	configureHealthAPI(app)
	configureBadgerLoadGeneratorAPIForAllPods(app)
	configureBadgerCompactionAPI(app, badgerLoadGenerator)
	return app
}

func configureBadgerCompactionAPI(app *iris.Application, badgerLoadGenerator *loadGenerators.BadgerLoadGenerator) {
	app.Get(badgerStartCompaction, func(ctx iris.Context) {

		go badgerLoadGenerator.BadgerCompaction()

		ctx.StatusCode(iris.StatusAccepted)
		_, err := ctx.WriteString("Started Compaction")
		if err != nil {
			zkLogger.ErrorF(LogTag, "Unable to write response %v", err)
			return
		}

	}).Describe("badger load generator")
}

func configureHealthAPI(app *iris.Application) {
	app.Get("/healthz", func(ctx iris.Context) {
		ctx.StatusCode(iris.StatusOK)
		ctx.WriteString("pong")
	}).Describe("healthcheck")
}

func configureBadgerLoadGeneratorAPIForAllPods(app *iris.Application) {
	app.Get(badgerLoadTestApiAllPods, func(ctx iris.Context) {

		// Scan all pods with the label
		podDetails := k8s.GetPodNameAndIPs(namespace, labelSelector)
		if len(podDetails) == 0 {
			podDetails = append(podDetails, k8s.PodDetails{Name: "localhost", IP: "localhost"})
		}
		zkLogger.InfoF(LogTag, "podDetails = %v", podDetails)

		traceCountPerPod, err := ctx.URLParamInt("traceCountPerPod")
		if traceCountPerPod == 0 || err != nil {
			traceCount, err1 := ctx.URLParamInt("traceCount")
			if traceCount == 0 || err1 != nil {
				traceCountPerPod = 2
				zkLogger.DebugF(LogTag, "zero or error traceCount = %v, traceCountPerPod=%v", traceCount, traceCountPerPod)
			} else {
				traceCountPerPod = traceCount / len(podDetails)
				zkLogger.DebugF(LogTag, "zero or error traceCount=%v, len(podDetails)=%v, traceCountPerPod = %v", traceCount, len(podDetails), traceCountPerPod)
			}
		}

		out := "accepted for all pods"
		for _, pod := range podDetails {

			url := fmt.Sprintf("http://%s:%d%s?traceCount=%d", pod.IP, port, badgerLoadTestApi, traceCountPerPod)
			zkLogger.InfoF(LogTag, "url = %s", url)

			out = fmt.Sprintf("%v\nPodName: %s, IP: %s, url:%s", out, pod.Name, pod.IP, url)

			//	make http call to the pod
			response, err1 := common.MakeHTTPCall(url)
			if err1 != nil {
				zkLogger.ErrorF(LogTag, "Error in making an http call %v", err1)
			}

			out = fmt.Sprintf("%v Status: %s", out, response)
		}

		ctx.StatusCode(iris.StatusAccepted)
		_, err = ctx.WriteString(out)
		if err != nil {
			zkLogger.ErrorF(LogTag, "Unable to write response %v", err)
			return
		}

	}).Describe("badger load generator")
}
