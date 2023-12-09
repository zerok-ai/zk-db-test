package main

import (
	"fmt"
	"github.com/kataras/iris/v12"
	zkConfig "github.com/zerok-ai/zk-utils-go/config"
	zkHttpConfig "github.com/zerok-ai/zk-utils-go/http/config"
	zkLogger "github.com/zerok-ai/zk-utils-go/logs"
	"os"
	"redis-test/config"
	"redis-test/internal/common"
	"redis-test/internal/k8s"
	loadGenerators "redis-test/internal/load-generators"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var LogTag = "main"

const (
	redisLoadTestApi        = "/gen-redis-load"
	redisLoadTestApiAllPods = "/gen-redis-load-all"

	namespace     = "zk-client"
	serviceName   = "zk-redis-test"
	labelSelector = "app=zk-redis-test"
	port          = 80
)

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

	redisLoadGenerator, err := loadGenerators.NewRedisLoadGenerator(cfg)
	if err != nil {
		panic(err)
	}
	defer redisLoadGenerator.Close()

	configurator := iris.WithConfiguration(iris.Configuration{
		DisablePathCorrection: true,
		LogLevel:              cfg.LogsConfig.Level,
	})
	if err = newApp(cfg, redisLoadGenerator).Listen(":"+cfg.Server.Port, configurator); err != nil {
		panic(err)
	}

}

func newApp(cfg config.AppConfigs, redisLoadGenerator *loadGenerators.RedisLoadGenerator) *iris.Application {
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
	configureRedisLoadGeneratorAPI(app, redisLoadGenerator)
	configureRedisLoadGeneratorAPIForAllPods(app)

	return app
}

func configureHealthAPI(app *iris.Application) {
	app.Get("/healthz", func(ctx iris.Context) {
		ctx.StatusCode(iris.StatusOK)
		ctx.WriteString("pong")
	}).Describe("healthcheck")
}

func configureRedisLoadGeneratorAPI(app *iris.Application, redisLoadGenerator *loadGenerators.RedisLoadGenerator) {
	app.Get(redisLoadTestApi, func(ctx iris.Context) {

		traceCount, err := ctx.URLParamInt("traceCount")
		if traceCount == 0 || err != nil {
			traceCount = 2
		}

		go redisLoadGenerator.GenerateLoad(traceCount)

		ctx.StatusCode(iris.StatusAccepted)
		_, err = ctx.WriteString("accepted")
		if err != nil {
			zkLogger.ErrorF(LogTag, "Unable to write response %v", err)
			return
		}
	}).Describe("redis load generator")
}

func configureRedisLoadGeneratorAPIForAllPods(app *iris.Application) {
	app.Get(redisLoadTestApiAllPods, func(ctx iris.Context) {

		// Scan all pods with the label
		podDetails := k8s.GetPodNameAndIPs(namespace, labelSelector)
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

			url := fmt.Sprintf("http://%s:%d%s?traceCount=%d", pod.IP, port, redisLoadTestApi, traceCountPerPod)
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

	}).Describe("redis load generator")
}
