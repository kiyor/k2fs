package lib

import (
	"bytes"
	// "flag" // Removed flag import
	"log"
	"time"

	"encoding/gob"

	"github.com/gomodule/redigo/redis"
	"github.com/kiyor/k2fs/pkg/core" // Added for core.GlobalAppConfig
)

var Redis *RedisPool
// var redisHost string // Removed package-level variable, will use core.GlobalAppConfig.RedisHost

// init() function with flag.StringVar removed.
// func init() {
// 	flag.StringVar(&redisHost, "redis", "192.168.10.10", "redis host")
// }

type RedisPool struct {
	Pool redis.Pool
}

func InitRedisPool() {
	// Ensure core.GlobalAppConfig.RedisHost is initialized before this is called.
	// Default value for RedisHost should be set in main.go's Cobra flags.
	if core.GlobalAppConfig.RedisHost == "" {
		log.Println("Warning: core.GlobalAppConfig.RedisHost is not set. Defaulting to localhost:6379 for Redis.")
		core.GlobalAppConfig.RedisHost = "localhost:6379" // Fallback, though Cobra default is better.
	}

	Redis = &RedisPool{
		redis.Pool{
			MaxIdle:     6,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				// Use core.GlobalAppConfig.RedisHost
				// The original code appended ":6379". If RedisHost includes port, this needs adjustment.
				// Assuming RedisHost will be like "hostname:port".
				redisAddr := core.GlobalAppConfig.RedisHost
				if !strings.Contains(redisAddr, ":") { // If port is not included, append default
					redisAddr += ":6379"
				}
				c, err := redis.Dial("tcp", redisAddr)
				if err != nil {
					return nil, err
				}
				return c, err
			},
			TestOnBorrow: func(c redis.Conn, t time.Time) error {
				_, err := c.Do("PING")
				return err
			},
		},
	}
	conn := Redis.Pool.Get()
	defer conn.Close()
	// Attempt to PING, log error if it occurs, but don't prevent startup.
	// The application might operate in a limited mode or fail later if Redis is critical.
	_, err := conn.Do("PING")
	if err != nil {
		log.Printf("Redis PING error on initial connection: %v. Check Redis server at %s.", err, core.GlobalAppConfig.RedisHost)
	} else {
		log.Printf("Successfully connected to Redis at %s", core.GlobalAppConfig.RedisHost)
	}
}

// Helper import for strings.Contains
import "strings"

func (r *RedisPool) Reset() {
	conn := r.Pool.Get()
	defer conn.Close()
	conn.Do("FLUSHALL")
}

func (r *RedisPool) Get(key string) ([]byte, bool) {
	conn := r.Pool.Get()
	defer conn.Close()
	res, err := conn.Do("GET", key)
	if err != nil {
		log.Println("redis", err)
		return nil, false
	}
	if res != nil {
		b := res.([]byte)
		return b, true
	} else {
		return nil, false
	}
}
func (r *RedisPool) GetValue(key string, value interface{}) bool {
	by, b := r.Get(key)
	if !b {
		return b
	}
	buf := bytes.NewBuffer(by)

	err := gob.NewDecoder(buf).Decode(value)
	if err != nil {
		log.Println(err)
	}
	return true
}

func (r *RedisPool) SetValueWithTTL(key string, value interface{}, second int) error {
	conn := r.Pool.Get()
	defer conn.Close()

	var buf bytes.Buffer
	gob.NewEncoder(&buf).Encode(value)

	_, err := conn.Do("SET", key, buf.Bytes())
	if err != nil {
		return err
	}
	_, err = conn.Do("EXPIRE", key, second)
	return err
}
