package handlers

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	zkLogger "github.com/zerok-ai/zk-utils-go/logs"
	zkconfig "github.com/zerok-ai/zk-utils-go/storage/redis/config"
	zktick "github.com/zerok-ai/zk-utils-go/ticker"
	"time"
)

var redisHandlerLogTag = "RedisHandler"

var (
	redisWriteCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method"},
	)
)

func init() {
	prometheus.MustRegister(redisWriteCounter)
}

type RedisHandler struct {
	RedisClient  *redis.Client
	ctx          context.Context
	config       *zkconfig.RedisConfig
	dbName       string
	Pipeline     redis.Pipeliner
	ticker       *zktick.TickerTask
	count        int
	startTime    time.Time
	batchSize    int
	syncInterval int
	tag          string
}

func NewRedisHandler(redisConfig *zkconfig.RedisConfig, dbName string, syncInterval int, batchSize int, tag string) (*RedisHandler, error) {
	handler := RedisHandler{
		ctx:    context.Background(),
		config: redisConfig,
		dbName: dbName,
	}

	err := handler.InitializeRedisConn()
	if err != nil {
		zkLogger.Error(redisHandlerLogTag, "Error while initializing redis connection ", err)
		return nil, err
	}

	handler.Pipeline = handler.RedisClient.Pipeline()

	timerDuration := time.Duration(syncInterval) * time.Millisecond
	handler.ticker = zktick.GetNewTickerTask("sync_pipeline", timerDuration, handler.SyncPipeline)
	handler.ticker.Start()

	handler.syncInterval = syncInterval
	handler.batchSize = batchSize
	handler.ctx = context.Background()
	handler.tag = tag

	return &handler, nil
}

func (h *RedisHandler) InitializeRedisConn() error {
	db := h.config.DBs[h.dbName]
	redisAddr := h.config.Host + ":" + h.config.Port
	opt := &redis.Options{
		Addr:     redisAddr,
		Password: h.config.Password,
		DB:       db,
	}
	redisClient := redis.NewClient(opt)

	h.RedisClient = redisClient
	err := h.PingRedis()
	if err != nil {
		return err
	}
	return nil
}

func (h *RedisHandler) Set(key string, value interface{}) error {
	statusCmd := h.RedisClient.Set(h.ctx, key, value, 0)
	return statusCmd.Err()
}

func (h *RedisHandler) SetNX(key string, value interface{}) error {
	statusCmd := h.RedisClient.SetNX(h.ctx, key, value, 0)
	return statusCmd.Err()
}

func (h *RedisHandler) HSet(key string, value interface{}) error {
	statusCmd := h.RedisClient.HSet(h.ctx, key, value, 0)
	return statusCmd.Err()
}

func (h *RedisHandler) HMSet(key string, value interface{}) error {
	statusCmd := h.RedisClient.HMSet(h.ctx, key, value)
	return statusCmd.Err()
}

func (h *RedisHandler) PingRedis() error {
	redisClient := h.RedisClient
	if redisClient == nil {
		zkLogger.Error(redisHandlerLogTag, "Redis client is nil.")
		return fmt.Errorf("redis client is nil")
	}
	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		zkLogger.Error(redisHandlerLogTag, "Error caught while pinging redis ", err)
		return err
	}
	return nil
}

func (h *RedisHandler) HMSetPipeline(key string, value map[string]string, expiration time.Duration) error {
	cmd := h.Pipeline.HMSet(h.ctx, key, value)
	if cmd.Err() != nil {
		return cmd.Err()
	}
	h.count++
	return h.setExpiry(key, expiration)
}

func (h *RedisHandler) SetNXPipeline(key string, value interface{}, expiration time.Duration) error {
	cmd := h.Pipeline.SetNX(h.ctx, key, value, expiration)
	if cmd.Err() != nil {
		return cmd.Err()
	}
	h.count++
	return h.setExpiry(key, expiration)
}

func (h *RedisHandler) SAddPipeline(key string, value interface{}, expiration time.Duration) error {
	cmd := h.Pipeline.SAdd(h.ctx, key, value)
	if cmd.Err() != nil {
		return cmd.Err()
	}
	h.count++
	return h.setExpiry(key, expiration)
}

func (h *RedisHandler) setExpiry(key string, expiration time.Duration) error {
	if expiration > 0 {
		cmd := h.Pipeline.Expire(h.ctx, key, expiration)
		if cmd.Err() != nil {
			return cmd.Err()
		}
	}
	return nil
}

func (h *RedisHandler) CheckRedisConnection() error {
	err := h.PingRedis()
	if err != nil {
		//Closing redis connection.
		err = h.CloseConnection()
		if err != nil {
			zkLogger.Error(redisHandlerLogTag, "Failed to close Redis connection: ", err)
			return err
		}
		err = h.InitializeRedisConn()
		if err != nil {
			zkLogger.Error(redisHandlerLogTag, "Error while initializing redis connection ", err)
			return err
		}
	}
	return nil
}

func (h *RedisHandler) SyncPipeline() {
	syncDuration := time.Duration(h.syncInterval) * time.Second

	count := h.count
	if count > h.batchSize || (count > 0 && time.Since(h.startTime) >= syncDuration) {
		_, err := h.Pipeline.Exec(h.ctx)
		if err != nil {
			zkLogger.Error(redisHandlerLogTag, "Error while syncing data to redis ", err)
			return
		}
		zkLogger.Debug(redisHandlerLogTag, "Pipeline synchronized. Batch size =", count)

		redisWriteCounter.WithLabelValues("redis-writes").Add(float64(count))

		h.count -= count
		h.startTime = time.Now()
	}
}

func (h *RedisHandler) CloseConnection() error {
	return h.RedisClient.Close()
}

func (h *RedisHandler) forceSync() {
	_, err := h.Pipeline.Exec(h.ctx)
	if err != nil {
		zkLogger.Error(redisHandlerLogTag, "Error while force syncing data to redis ", err)
		return
	}
}

func (h *RedisHandler) shutdown() {
	h.forceSync()
	err := h.CloseConnection()
	if err != nil {
		zkLogger.Error(redisHandlerLogTag, "Error while closing redis conn.")
		return
	}
}
