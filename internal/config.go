package internal

import (
	"github.com/spf13/viper"
)

type Config struct {
	// server port
	Port int `yaml:"port"`
	// 单位 秒
	Heartbeat int `yaml:"heartbeat"`
	// 桶越多, 推送的锁粒度越小, 推送并发度越高
	BucketCount int `yaml:"bucketCount"`
	// 每个桶的处理协程数量,影响同一时刻可以有多少个不同消息被分发出去
	BucketWorkerCount int `yaml:"bucketWorkerCount"`
	// 待分发队列的长度，分发队列缓冲所有待推送的消息, 等待被分发到Bucket
	DispatchChannelSize int `yaml:"dispatchChannelSize"`
	// 合并推送的最大延迟时间，单位毫秒, 在抵达maxPushBatchSize之前超时则发送
	MaxMergerDelay int `yaml:"maxMergerDelay"`
	// 合并最多消息条数，消息推送频次越高, 应该使用更大的合并批次, 得到更高的吞吐收益
	MaxMergerBatchSize int `yaml:"maxMergerBatchSize"`
	// 消息合并协程的数量，消息合并与json编码耗费CPU, 注意一个房间的消息只会由同一个协程处理
	MergerWorkerCount int `yaml:"mergerWorkerCount"`
	// 消息合并队列的容量，每个房间消息合并线程有一个队列, 推送量超过队列将被丢弃
	MergerChannelSize int `yaml:"mergerChannelSize"`
	// 每个连接最多加入房间数量，目前房间ID没有校验, 所以先做简单的数量控制
	MaxJoinRoom int `yaml:"maxJoinRoom"`
	// 分发协程的数量，分发协程用于将待推送消息扇出给各个Bucket
	DispatchWorkerCount int `yaml:"dispatchWorkerCount"`
	// Bucket工作队列长度，每个Bucket的分发任务放在一个独立队列中
	BucketJobChannelSize int `yaml:"bucketJobChannelSize"`
	// Bucket发送协程的数量，每个Bucket有多个协程并发的推送消息
	BucketJobWorkerCount int `yaml:"bucketJobWorkerCount"`
}

//var defaultConfig = Config{
//	Port:                 7000,
//	Heartbeat:            60,
//	BucketCount:          512,
//	BucketWorkerCount:    32,
//	DispatchChannelSize:  10000,
//	MaxMergerDelay:       1000,
//	MaxMergerBatchSize:   100,
//	MergerWorkerCount:    32,
//	MergerChannelSize:    1000,
//	MaxJoinRoom:          5,
//	DispatchWorkerCount:  32,
//	BucketJobChannelSize: 1000,
//	BucketJobWorkerCount: 32,
//}

var GlobalConfig *Config

func InitConfig(filename string) (*Config, error) {
	GlobalConfig = &Config{}

	viper.SetConfigType("yaml")
	viper.SetConfigFile(filename)
	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	err = viper.Unmarshal(GlobalConfig)
	return GlobalConfig, err
}
