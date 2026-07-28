package handlers

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisClient Redis客户端
var RedisClient *redis.Client
var ctx = context.Background()

// InitRedis 初始化Redis连接
func InitRedis(addr, password string, db int) error {
	RedisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// 测试连接
	_, err := RedisClient.Ping(ctx).Result()
	if err != nil {
		log.Printf("Failed to connect to Redis: %v", err)
		return err
	}

	log.Println("Successfully connected to Redis")
	return nil
}

// SetCache 设置缓存
func SetCache(key string, value interface{}, expiration time.Duration) error {
	if RedisClient == nil {
		return nil
	}

	data, err := json.Marshal(value)
	if err != nil {
		log.Printf("Failed to marshal cache value: %v", err)
		return err
	}

	return RedisClient.Set(ctx, key, data, expiration).Err()
}

// GetCache 获取缓存
func GetCache(key string, dest interface{}) error {
	if RedisClient == nil {
		return redis.Nil
	}

	data, err := RedisClient.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}

	return json.Unmarshal(data, dest)
}

// DeleteCache 删除缓存
func DeleteCache(key string) error {
	if RedisClient == nil {
		return nil
	}

	return RedisClient.Del(ctx, key).Err()
}

// ClearCache 清空缓存
func ClearCache(pattern string) error {
	if RedisClient == nil {
		return nil
	}

	keys, err := RedisClient.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}

	if len(keys) > 0 {
		return RedisClient.Del(ctx, keys...).Err()
	}

	return nil
}
