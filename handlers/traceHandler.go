package handlers

import (
	"encoding/json"
	"fmt"
	logger "github.com/zerok-ai/zk-utils-go/logs"
	"github.com/zerok-ai/zk-utils-go/storage/badger"
	zktick "github.com/zerok-ai/zk-utils-go/ticker"
	"math/rand"
	"redis-test/config"
	"redis-test/model"
	"sync"
	"time"
)

var traceLogTag = "TraceHandler"
var delimiter = "-"

const (
	logInterval = 1 // Log every 5 seconds
)

type TraceHandler struct {
	traceBadgerHandler *badger.BadgerStoreHandler
	traceStoreMutex    sync.Mutex
	traceStore         sync.Map
}

func NewTraceHandler(config *config.AppConfigs) (*TraceHandler, error) {

	traceBadgerHandler, err := badger.NewBadgerHandler(&config.Badger)
	if err != nil {
		logger.Error(traceLogTag, "Error while creating badger handler:", err)
		return nil, err
	}

	handler := TraceHandler{traceBadgerHandler: traceBadgerHandler}

	//read data from badger for every 5 mins and log the count
	var readTicker *zktick.TickerTask
	timerDuration := time.Duration(config.Badger.GCTimerDuration) * time.Second
	readTicker = zktick.GetNewTickerTask("badger_log_data_count", timerDuration, func() {
		LogDataFromDB(traceBadgerHandler)
	})
	readTicker.Start()

	return &handler, nil
}

// Populate Span common properties.
func (th *TraceHandler) createSpanDetails(parentSpanId string) model.OTelSpanDetails {
	scopeAttributes := model.GenericMapPtrFromMap(make(map[string]interface{}))
	spanAttributes := model.GenericMapPtrFromMap(map[string]interface{}{
		"db.name":              "productservice",
		"db.user":              "service_user",
		"db.system":            "mysql",
		"thread.id":            1,
		"thread.name":          "main",
		"db.operation":         "SELECT",
		"db.sql.table":         "productservice",
		"db.statement":         "SHOW FULL TABLES FROM `productservice` LIKE ?",
		"net.peer.name":        "mysql-svc.mysql.svc.cluster.local",
		"net.peer.port":        3306,
		"db.connection_string": "mysql://mysql-svc.mysql.svc.cluster.local:3306",
	})

	resourceAttributes := model.GenericMapPtrFromMap(map[string]interface{}{
		"os.type":                     "linux",
		"host.arch":                   "amd64",
		"host.name":                   "order-6bfc474747-w9mbc",
		"process.pid":                 1,
		"container.id":                "5956c751c51121a467d29cb265eb169970e6aba70bd30a3427bbca8ccea52d39",
		"k8s.pod.name":                "order-6bfc474747-w9mbc",
		"service.name":                "order",
		"k8s.node.name":               "gke-devclient03-default-pool-3cfcf5c9-fhkc",
		"os.description":              "Linux 5.15.133+",
		"service.version":             "configurable",
		"k8s.container.name":          "order",
		"k8s.namespace.name":          "sofa-shop-mysql",
		"telemetry.sdk.name":          "opentelemetry",
		"k8s.deployment.name":         "order",
		"k8s.replicaset.name":         "order-6bfc474747",
		"process.runtime.name":        "OpenJDK Runtime Environment",
		"telemetry.sdk.version":       "1.30.1",
		"telemetry.auto.version":      "1.30.0",
		"telemetry.sdk.language":      "java",
		"process.executable.path":     "/usr/local/openjdk-17/bin/java",
		"process.runtime.version":     "17.0.2+8-86",
		"process.runtime.description": "Oracle Corporation OpenJDK 64-Bit Server VM 17.0.2+8-86",
	})

	sourceIp := "10.60.1.19"
	destIp := "10.60.0.53"
	jsonData := `[{"error_type":"exception","exception_type":"java.lang.IllegalArgumentException","hash":"6995586f1dc5c1002c21d359cd81052cf103925063135bb237ba6950b1161623","message":"item not in stock"}]`

	// Unmarshal JSON data into a slice of SpanErrorInfo
	var errors []model.SpanErrorInfo
	err := json.Unmarshal([]byte(jsonData), &errors)
	if err != nil {
		logger.Error(traceLogTag, "Error while unmarshalling error data ", err)
	}
	source := "ingress-nginx-controller-7d884d97b5"
	spanDetail := model.OTelSpanDetails{
		SpanKind:           "SERVER",
		ServiceName:        "zk-db-test",
		SpanName:           "test-span",
		StartNs:            uint64(time.Now().UnixNano()),
		Protocol:           "HTTP",
		Destination:        nil,
		Source:             &source,
		LatencyNs:          011223344,
		SpanAttributes:     spanAttributes,
		Scheme:             nil,
		Path:               nil,
		Query:              nil,
		Status:             nil,
		Username:           nil,
		SourceIp:           &sourceIp,
		DestIp:             &destIp,
		Method:             nil,
		Route:              nil,
		ScopeAttributes:    scopeAttributes,
		Errors:             errors,
		ResourceAttributes: resourceAttributes,
		WorkloadIdList:     []string{"4dfca142-5bf8-5762-94ac-9bf5300f985c"},
	}
	spanDetail.SetParentSpanId(parentSpanId)
	return spanDetail
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

func (th *TraceHandler) StartBadgerCompaction() {
	th.traceBadgerHandler.StartCompaction()
}

func LogDataFromDB(badger *badger.BadgerStoreHandler) {
	////logs data from badger for every 5 mins
	//count, err := GetTotalDataCount(badger)
	//if err != nil {
	//	fmt.Printf("Error while getting total data count: %s\n", err)
	//}
	//pair, s, err := GetAnyKeyValuePair(badger)
	//if err != nil {
	//	fmt.Printf("Error while getting any key value pair: %s\n", err)
	//}
	//fmt.Printf("Total data count: %s, badger random key value pair: %s, %s\n", count, pair, s)

	stringArray := []string{""}
	keyVals, err := badger.BulkGetForPrefix(stringArray)
	if err != nil {
		fmt.Printf("Error while getting bulk data: %s\n", err)
	}
	for key, value := range keyVals {
		fmt.Printf("Key: %s, Value: %s\n", key, value)
	}

}
