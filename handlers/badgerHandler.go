package handlers

import (
	"context"
	"encoding/json"
	"github.com/dgraph-io/badger/v4"
	"github.com/prometheus/client_golang/prometheus"
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

var (
	badgerObjectsCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zk_badger_objects",
			Help: "Total number of objects written in badger",
		},
		[]string{"method"},
	)

	badgerWritesCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "zk_badger_writes",
			Help: "Total number of writes to badger",
		},
		[]string{"method"},
	)
)

const badgerHandlerLogTag = "BadgerHandler"
const badgerDbPath = "/zerok/badger-db"

type BadgerHandler struct {
	ctx          context.Context
	config       *config.AppConfigs
	dbPath       string
	ticker       *zktick.TickerTask
	syncInterval int
	batchSize    int
	db           *badger.DB
	wb           *badger.WriteBatch
	count        int
	startTime    time.Time
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

	////check badger connection
	//if err := h.CheckRedisConnection(); err != nil {
	//	zkLogger.Error(badgerHandlerLogTag, "Error while checking badger conn ", err)
	//	return err
	//}

	spanJSON, err := json.Marshal(spanDetails)
	if err != nil {
		zkLogger.Debug(badgerHandlerLogTag, "Error encoding SpanDetails for spanID %s: %v\n", spanId, err)
		return err
	}
	spanJsonStr := string(spanJSON)
	newTraceId := traceId + delimiter + spanId
	err = b.HMSetBadgerPipeline(newTraceId, spanJsonStr, time.Duration(b.config.Traces.Ttl)*time.Second)
	if err != nil {
		zkLogger.Error(badgerHandlerLogTag, "Error while setting trace details for traceId %s: %v\n", traceId, err)
		return err
	}
	return nil
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
	db, err := badger.Open(badger.DefaultOptions(badgerDbPath))
	if err != nil {
		zkLogger.Error(badgerHandlerLogTag, "Error while initializing connection ", err)
		return err
	}

	wb := db.NewWriteBatch()
	b.wb = wb
	b.db = db
	return nil
}

func (b *BadgerHandler) Close() error {
	return b.db.Close()
}

func (b *BadgerHandler) HMSetBadgerPipeline(key string, value string, expiration time.Duration) error {
	newValue, _ := json.Marshal(value)

	if err := b.wb.SetEntry(badger.NewEntry([]byte(key), newValue).
		WithTTL(expiration)); err != nil {
		return err
	}
	b.count++
	if b.count > b.batchSize {
		b.SyncPipeline()
	}
	return nil
}

func (b *BadgerHandler) SyncPipeline() {
	syncDuration := time.Duration(b.syncInterval) * time.Second
	count := b.count
	if count > b.batchSize || (count > 0 && time.Since(b.startTime) >= syncDuration) {
		err := b.wb.Flush()
		if err != nil {
			zkLogger.Error(badgerHandlerLogTag, "Error while syncing data to Badger ", err)
			return
		}
		//request counter to badger DB
		requestCounter++

		badgerWritesCounter.WithLabelValues("badger-writes").Inc()
		badgerObjectsCounter.WithLabelValues("badger-writes").Add(float64(count))

		zkLogger.Debug(badgerHandlerLogTag, "Pipeline synchronized. event sent. Batch size =", count)

		b.count -= count
		b.startTime = time.Now()
	}
}

func (b *BadgerHandler) CloseDbConnection() error {
	err := b.db.Close()
	if err != nil {
		return err
	}
	return nil
}
