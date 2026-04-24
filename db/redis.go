package db

import (
	"os"

	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client

func ConnectRedis() {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	Redis = redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})
}
