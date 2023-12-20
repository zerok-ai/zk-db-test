package handlers

import (
	"context"
	"encoding/json"
	badger "github.com/dgraph-io/badger/v4"
	zkLogger "github.com/zerok-ai/zk-utils-go/logs"
	zktick "github.com/zerok-ai/zk-utils-go/ticker"
	"redis-test/config"
	"redis-test/model"
	"time"
)

type SpanInBadger struct {
	SpanId      string                `json:"id"`
	SpanDetails model.OTelSpanDetails `json:"sd"`
}

type TraceInBadger struct {
	Spans []SpanInBadger `json:"sps"`
}

const badgerHandlerLogTag = "BadgerHandler"

type BadgerHandler struct {
	ctx          context.Context
	config       *config.AppConfigs
	dbPath       string
	ticker       *zktick.TickerTask
	syncInterval int
	batchSize    int
	db           *badger.DB
}

func NewBadgerHandler(configs *config.AppConfigs, dbName string) (DBHandler, error) {
	handler := BadgerHandler{
		ctx:    context.Background(),
		config: configs,
		dbPath: dbName,
	}

	err := handler.InitializeConn()
	if err != nil {
		zkLogger.Error(badgerHandlerLogTag, "Error while initializing connection ", err)
		return nil, err
	}

	syncInterval := configs.Traces.SyncDurationMS
	timerDuration := time.Duration(syncInterval) * time.Millisecond
	handler.ticker = zktick.GetNewTickerTask("sync_pipeline", timerDuration, handler.SyncPipeline)
	handler.ticker.Start()

	handler.syncInterval = syncInterval
	handler.batchSize = configs.Traces.SyncBatchSize
	handler.ctx = context.Background()

	return &handler, nil
}

func (b *BadgerHandler) PutTraceData(traceId string, spanId string, spanDetails model.OTelSpanDetails) error {
	//TODO implement me
	panic("implement me")
}

func (b *BadgerHandler) SyncPipeline() {

	//key := []byte("merge")
	//
	//m := db.GetMergeOperator(key, add, 200*time.Millisecond)
	//defer m.Stop()
	//
	//m.Add([]byte("A"))
	//m.Add([]byte("B"))
	//m.Add([]byte("C"))

	//TODO implement me
	panic("implement me")
}

// Merge function to append one byte slice to another
func mergeSpansOfTrace(originalValue, newValue []byte) []byte {

	if newValue == nil {
		return originalValue
	}

	// 1. Unmarshal old trace data
	var trace TraceInBadger
	if originalValue != nil {
		err := json.Unmarshal(originalValue, &trace)
		if err != nil {
			originalValue = nil
			zkLogger.Error(badgerHandlerLogTag, "Error while unmarshalling old trace data ", err)
		}
	}

	if originalValue == nil {
		trace = TraceInBadger{}
	}

	// 2. Unmarshal and merge new trace data
	var newTrace TraceInBadger
	err := json.Unmarshal(newValue, &newTrace)
	if err != nil {
		zkLogger.Error(badgerHandlerLogTag, "Error while unmarshalling old trace data ", err)
		return originalValue
	}

	// merge spans
	trace.Spans = append(trace.Spans, newTrace.Spans...)

	// Marshal trace data
	b, err := json.Marshal(trace)
	if err != nil {
		zkLogger.Error(badgerHandlerLogTag, "Error while marshalling trace data ", err)
		return nil
	}

	return b
}

func (b *BadgerHandler) InitializeConn() error {

	// Open the Badger database located in the /tmp/badger directory.
	// It will be created if it doesn't exist.
	db, err := badger.Open(badger.DefaultOptions("/tmp/badger"))
	if err != nil {
		zkLogger.Error(badgerHandlerLogTag, "Error while initializing connection ", err)
		return err
	}

	b.db = db
	return nil
}

func (b *BadgerHandler) Close() error {
	return b.db.Close()
}
