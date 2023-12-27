package handlers

import (
	"fmt"
	logger "github.com/zerok-ai/zk-utils-go/logs"
	"github.com/zerok-ai/zk-utils-go/storage/redis/clientDBNames"
	"math/rand"
	"redis-test/config"
	"redis-test/model"
	"sync"
	"time"
)

var traceLogTag = "TraceHandler"
var delimiter = "__"

type DBHandler interface {
	PutTraceData(traceId string, spanId string, spanDetails model.OTelSpanDetails) error
	SyncPipeline()
	CloseDbConnection() error
}

type TraceHandler struct {
	traceRedisHandler  DBHandler
	traceBadgerHandler DBHandler

	traceStoreMutex sync.Mutex
	traceStore      sync.Map
}

func NewTraceHandler(config *config.AppConfigs) (*TraceHandler, error) {

	redisHandler, err := NewRedisHandler(config, clientDBNames.TraceDBName)
	if err != nil {
		logger.Error(traceLogTag, "Error while creating redis handler:", err)
		return nil, err
	}

	badgerHandler, err := NewBadgerHandler(config, clientDBNames.TraceDBName)
	if err != nil {
		logger.Error(traceLogTag, "Error while creating badger handler:", err)
		return nil, err
	}

	handler := TraceHandler{traceRedisHandler: redisHandler, traceBadgerHandler: badgerHandler}
	return &handler, nil
}

func (th *TraceHandler) PushDataToRedis(runId string, traceCount, spanCountPerTrace int) {

	for traceIndex := 0; traceIndex < traceCount; traceIndex++ {
		traceIDStr := fmt.Sprintf("00-aaaa%s", generateRandomHex(28))

		parentSpanId := "0000000000000000"
		for spanIndex := 0; spanIndex < spanCountPerTrace; spanIndex++ {

			spanDetails := th.createSpanDetails(parentSpanId)

			// Generate a random span ID (16 characters)
			spanID := generateRandomHex(16)

			err := th.traceRedisHandler.PutTraceData(traceIDStr, spanID, spanDetails)
			if err != nil {
				logger.Debug(traceLogTag, "Error while putting trace data to redis ", err)
				return
			}

			parentSpanId = spanID
		}
	}

	th.traceRedisHandler.SyncPipeline()
}

func (th *TraceHandler) PushDataToBadger(runId string, traceCount, spanCountPerTrace int) {

	for traceIndex := 0; traceIndex < traceCount; traceIndex++ {
		traceIDStr := fmt.Sprintf("00-aaaa%s", generateRandomHex(28))

		parentSpanId := "0000000000000000"
		for spanIndex := 0; spanIndex < spanCountPerTrace; spanIndex++ {

			spanDetails := th.createSpanDetails(parentSpanId)

			// Generate a random span ID (16 characters)
			spanID := generateRandomHex(16)

			err := th.traceBadgerHandler.PutTraceData(traceIDStr, spanID, spanDetails)
			if err != nil {
				logger.Debug(traceLogTag, "Error while putting trace data to redis ", err)
				return
			}

			parentSpanId = spanID
		}
	}

	th.traceBadgerHandler.SyncPipeline()
}

// Populate Span common properties.
func (th *TraceHandler) createSpanDetails(parentSpanId string) model.OTelSpanDetails {
	spanDetail := model.OTelSpanDetails{}
	spanDetail.SetParentSpanId(parentSpanId)
	return spanDetail
}

func (th *TraceHandler) CloseBadgerConnection() {
	th.traceBadgerHandler.CloseDbConnection()
}

// Function to generate a random hexadecimal string of a given length
func generateRandomHex(length int) string {
	rand.Seed(time.Now().UnixNano())
	hexChars := "0123456789abcdef"
	result := make([]byte, length)
	for i := 0; i < length; i++ {
		result[i] = hexChars[rand.Intn(len(hexChars))]
	}
	return string(result)
}
