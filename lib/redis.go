package lib

import (
	"bytes"
	"flag"
	"log"
	"time"

	"encoding/gob"

	"github.com/gomodule/redigo/redis"
)

var Redis *RedisPool
var redisHost string

func init() {
	flag.StringVar(&redisHost, "redis", "192.168.10.10", "redis host")
}

type RedisPool struct {
	Pool redis.Pool
}

func InitRedisPool() {
	Redis = &RedisPool{
		redis.Pool{
			MaxIdle:     6,
			IdleTimeout: 240 * time.Second,
			Dial: func() (redis.Conn, error) {
				c, err := redis.Dial("tcp", redisHost+":6379")
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
	_, err := conn.Do("PING")
	if err != nil {
		log.Println("redis conn error:", err)
	}
}

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
