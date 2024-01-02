package load_generators

import (
	"github.com/google/uuid"
	zkLogger "github.com/zerok-ai/zk-utils-go/logs"
	"redis-test/handlers"
)

import (
	"redis-test/config"
)

const (
	traceBadgerHandlerLogTag = "TraceBadgerHandler"
)

type BadgerLoadGenerator struct {
	id           string
	cfg          config.AppConfigs
	traceHandler *handlers.TraceHandler
}

func (badgerLoadGenerator BadgerLoadGenerator) Close() {
	err := badgerLoadGenerator.traceHandler.CloseBadgerConnection()
	if err != nil {
		zkLogger.Error(traceBadgerHandlerLogTag, "Error while closing badger connection ", err)
		return
	}
}

func NewBadgerLoadGenerator(cfg config.AppConfigs, traceHandler *handlers.TraceHandler) (*BadgerLoadGenerator, error) {

	fp := BadgerLoadGenerator{
		id:           "BLG" + uuid.New().String(),
		traceHandler: traceHandler,
		cfg:          cfg,
	}
	return &fp, nil
}

func (badgerLoadGenerator BadgerLoadGenerator) GenerateLoad(traceCount int) {
	runId := uuid.New().String()
	badgerLoadGenerator.traceHandler.PushDataToBadger(runId, traceCount, spansPerTrace)
}

func (badgerLoadGenerator BadgerLoadGenerator) LogDBRequestsLoad() {
	badgerLoadGenerator.traceHandler.LogDBRequestsLoad()
}
