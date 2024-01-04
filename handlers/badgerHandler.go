package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"github.com/kataras/iris/v12/x/errors"
	"github.com/prometheus/client_golang/prometheus"
	zkLogger "github.com/zerok-ai/zk-utils-go/logs"
	zktick "github.com/zerok-ai/zk-utils-go/ticker"
	"log"
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
	requestCounter int64
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
	timerDuration := 300 * time.Second
	handler.ticker = zktick.GetNewTickerTask("badger_garbage_collect", timerDuration, handler.GarbageCollect)
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
	//	zkLogger.Error(badgerHandlerLogTag, "Error while checking badger conn ", err
	//	return err
	//

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
	db, err := badger.Open(badger.DefaultOptions(badgerDbPath).WithValueLogFileSize(1 << 26))
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
	//syncDuration := time.Duration(b.syncInterval) * time.Second
	count := b.count
	//|| (count > 0 && time.Since(b.startTime) >= syncDuration)
	if count > b.batchSize {
		err := b.wb.Flush()
		if err != nil {
			zkLogger.Error(badgerHandlerLogTag, "Error while syncing data to Badger ", err)
			return
		}
		//refreshing the write batch for new batch to overcome  Batch commit not permitted after finish error
		b.wb = b.db.NewWriteBatch()
		//request counter to badger DB
		requestCounter++

		badgerWritesCounter.WithLabelValues("badger-writes").Inc()
		badgerObjectsCounter.WithLabelValues("badger-writes").Add(float64(count))

		//zkLogger.Debug(badgerHandlerLogTag, "Pipeline synchronized. event sent. Batch size =", count)

		b.count -= count
		b.startTime = time.Now()
	}
}

func (b *BadgerHandler) GarbageCollect() {

	err := b.db.RunValueLogGC(0.001)
	if err != nil {
		if errors.Is(err, badger.ErrNoRewrite) {
			zkLogger.Debug(badgerHandlerLogTag, "No BadgerDB GC occurred:", err)
		} else {
			zkLogger.Error(badgerHandlerLogTag, "Error while running garbage collector ", err)
		}
	}
}

func (b *BadgerHandler) CloseDbConnection() error {
	err := b.db.Close()
	if err != nil {
		return err
	}
	return nil
}

func (b *BadgerHandler) LogDBRequestsLoad() {
	for {
		time.Sleep(logInterval * time.Second)
		currentCount := requestCounter
		if requestCounter == 0 {
			break
		}
		requestCounter = 0 // Reset counter for the next interval
		log.Printf("Requests per second: %d", currentCount/logInterval)
	}
}

func (b *BadgerHandler) GetData(id string) (string, error) {

	badgerDbValue := ""
	err := b.db.View(func(txn *badger.Txn) error {
		key := []byte(id)
		item, err := txn.Get(key)
		if err != nil {
			return err
		}

		// Accessing the value
		var value []byte
		err = item.Value(func(val []byte) error {
			value = append([]byte{}, val...)
			return nil
		})

		if err != nil {
			return err
		}

		fmt.Printf("Key: %s, Value: %s\n", key, value)
		badgerDbValue = string(value) // Assign the value to temp
		return nil
	})

	if err != nil {
		return "", err
	}

	return badgerDbValue, nil
}

func (b *BadgerHandler) GetAnyKeyValuePair() (string, string, error) {
	var key, value string

	err := b.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		// Move to the first key-value pair
		it.Rewind()
		if it.Valid() {
			item := it.Item()
			key = string(item.Key())
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			value = string(val)
		} else {
			return fmt.Errorf("database is empty")
		}

		return nil
	})

	if err != nil {
		return "", "", err
	}

	return key, value, nil
}

func (b *BadgerHandler) GetTotalDataCount() (string, error) {
	// Iterate over the keys in the database and count them
	// Count of keys in memory
	memoryCount := 0
	err := b.db.View(func(txn *badger.Txn) error {
		// Create an iterator for in-memory keys
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			memoryCount++
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	// Count of keys on disk
	diskCount := 0
	err = b.db.View(func(txn *badger.Txn) error {
		// Create an iterator for keys on disk
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false // Only fetch keys, not values
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			diskCount++
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	diskCountString := fmt.Sprintf("Memory count: %d, Disk count: %d", memoryCount, diskCount)
	return diskCountString, nil

}

func (b *BadgerHandler) StartCompaction() {
	err := b.db.Flatten(2)
	if err != nil {
		if errors.Is(err, badger.ErrNoRewrite) {
			zkLogger.Debug(badgerHandlerLogTag, "No BadgerDB GC occurred:", err)
		} else {
			zkLogger.Error(badgerHandlerLogTag, "Error while running garbage collector ", err)
		}
	}
}
