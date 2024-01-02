package load_generators

import (
	"github.com/google/uuid"
	"redis-test/config"
	"redis-test/handlers"
)

const (
	traceRedisHandlerLogTag = "TraceRedisHandler"
	spansPerTrace           = 10
)

type RedisLoadGenerator struct {
	id           string
	cfg          config.AppConfigs
	traceHandler *handlers.TraceHandler
}

func (redisLoadGenerator RedisLoadGenerator) Close() {

}

func NewRedisLoadGenerator(cfg config.AppConfigs, traceHandler *handlers.TraceHandler) (*RedisLoadGenerator, error) {

	fp := RedisLoadGenerator{
		id:           "RLG" + uuid.New().String(),
		traceHandler: traceHandler,
		cfg:          cfg,
	}
	return &fp, nil
}

func (redisLoadGenerator RedisLoadGenerator) GenerateLoad(traceCount int) {
	runId := uuid.New().String()
	redisLoadGenerator.traceHandler.PushDataToRedis(runId, traceCount, spansPerTrace)
}
