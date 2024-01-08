package handlers

import (
	"context"
	"github.com/prometheus/client_golang/prometheus"
	logger "github.com/zerok-ai/zk-utils-go/logs"
	"github.com/zerok-ai/zk-utils-go/storage/badger"
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
	requestCounter int64
)

const badgerHandlerLogTag = "BadgerHandler"
const badgerDbPath = "/zerok/badger-db"

var traceBadgerHandlerLogTag = "TraceBadgerHandler"

type BadgerHandler struct {
	badgerHandler *badger.BadgerStoreHandler
	ctx           context.Context
	config        *config.AppConfigs
}

func NewTracesBadgerHandler(app *config.AppConfigs) (*BadgerHandler, error) {
	badgerHandler, err := badger.NewBadgerHandler(&app.Badger)

	if err != nil {
		logger.Error(traceBadgerHandlerLogTag, "Error while creating badger client ", err)
	}

	//read data from badger for every 5 mins and log the count
	var readTicker *zktick.TickerTask
	readTicker = zktick.GetNewTickerTask("badger_log_data_count", app.Badger.GCTimerDuration, func() {
		LogDataFromDB(badgerHandler)
	})
	readTicker.Start()
	handler := &BadgerHandler{
		badgerHandler: badgerHandler,
		ctx:           context.Background(),
		config:        app,
	}
	return handler, nil
}

func (h *BadgerHandler) PutTraceData(traceId string, spanId string, spanJSON string) error {
	key := traceId + "-" + spanId

	if err := h.badgerHandler.Set(key, spanJSON, int64(time.Duration(h.config.Traces.Ttl)*time.Second)); err != nil {
		logger.Error(traceBadgerHandlerLogTag, "Error while setting trace details for traceId %s: %v\n", traceId, err)
		return err
	}
	return nil
}

func (h *BadgerHandler) SyncPipeline() {
	h.badgerHandler.StartCompaction()
}

//func GetTotalDataCount(badger *badger.BadgerStoreHandler) (string, error) {
//	// Iterate over the keys in the database and count them
//	// Count of keys in memory
//	memoryCount := 0
//	err := badger.db.View(func(txn *badger.Txn) error {
//		// Create an iterator for in-memory keys
//		it := txn.NewIterator(badger.DefaultIteratorOptions)
//		defer it.Close()
//
//		for it.Rewind(); it.Valid(); it.Next() {
//			memoryCount++
//		}
//		return nil
//	})
//	if err != nil {
//		return "", err
//	}
//
//	// Count of keys on disk
//	diskCount := 0
//	err = b.db.View(func(txn *badger.Txn) error {
//		// Create an iterator for keys on disk
//		opts := badger.DefaultIteratorOptions
//		opts.PrefetchValues = false // Only fetch keys, not values
//		it := txn.NewIterator(opts)
//		defer it.Close()
//
//		for it.Rewind(); it.Valid(); it.Next() {
//			diskCount++
//		}
//		return nil
//	})
//	if err != nil {
//		return "", err
//	}
//	diskCountString := fmt.Sprintf("Memory count: %d, Disk count: %d", memoryCount, diskCount)
//	return diskCountString, nil
//
//}
//
//func GetAnyKeyValuePair(badger *badger.BadgerStoreHandler) (string, string, error) {
//	var key, value string
//
//	err := b.db.View(func(txn *badger.Txn) error {
//		opts := badger.DefaultIteratorOptions
//		it := txn.NewIterator(opts)
//		defer it.Close()
//
//		// Move to the first key-value pair
//		it.Rewind()
//		if it.Valid() {
//			item := it.Item()
//			key = string(item.Key())
//			val, err := item.ValueCopy(nil)
//			if err != nil {
//				return err
//			}
//			value = string(val)
//		} else {
//			return fmt.Errorf("database is empty")
//		}
//
//		return nil
//	})
//
//	if err != nil {
//		return "", "", err
//	}
//
//	return key, value, nil
//}
