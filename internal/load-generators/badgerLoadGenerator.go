package load_generators

import (
	"github.com/google/uuid"
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

}

func NewBadgerLoadGenerator(cfg config.AppConfigs) (*BadgerLoadGenerator, error) {

	traceHandler, err := handlers.NewTraceHandler(&cfg)
	if err != nil {
		return nil, err
	}

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
