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
	spansPerTrace            = 10
)

type BadgerLoadGenerator struct {
	id           string
	cfg          config.AppConfigs
	traceHandler *handlers.TraceHandler
}

func NewBadgerLoadGenerator(cfg config.AppConfigs, traceHandler *handlers.TraceHandler) (*BadgerLoadGenerator, error) {

	fp := BadgerLoadGenerator{
		id:           "BLG" + uuid.New().String(),
		traceHandler: traceHandler,
		cfg:          cfg,
	}
	return &fp, nil
}

func (badgerLoadGenerator BadgerLoadGenerator) BadgerCompaction() {
	badgerLoadGenerator.traceHandler.StartBadgerCompaction()
}
