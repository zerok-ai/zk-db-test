package config

import (
	zkHttpConfig "github.com/zerok-ai/zk-utils-go/http/config"
	zkLogsConfig "github.com/zerok-ai/zk-utils-go/logs/config"
	badger "github.com/zerok-ai/zk-utils-go/storage/badger/config"
	storage "github.com/zerok-ai/zk-utils-go/storage/redis/config"
)

type ServerConfig struct {
	Host string `yaml:"host" env:"SRV_HOST,HOST" env-description:"Server host" env-default:"localhost"`
	Port string `yaml:"port" env:"SRV_PORT,PORT" env-description:"Server port" env-default:"80"`
}

type TraceConfig struct {
	SyncDurationMS int `yaml:"syncDurationMS"`
	SyncBatchSize  int `yaml:"syncBatchSize"`
	Ttl            int `yaml:"ttl"`
}

type BadgerConfig struct {
	FilePath string `yaml:"filePath"`
}

// AppConfigs is an application configuration structure
type AppConfigs struct {
	Redis      storage.RedisConfig     `yaml:"redis"`
	Badger     badger.BadgerConfig     `yaml:"badger"`
	Server     ServerConfig            `yaml:"server"`
	Traces     TraceConfig             `yaml:"traces"`
	LogsConfig zkLogsConfig.LogsConfig `yaml:"logs"`
	Http       zkHttpConfig.HttpConfig `yaml:"http"`
	Greeting   string                  `env:"GREETING" env-description:"Greeting phrase" env-default:"Hello!"`
}
