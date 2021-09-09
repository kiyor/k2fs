package main

import (
	"time"

	"github.com/bluele/gcache"
)

var cache = gcache.New(2000).LRU().Expiration(5 * time.Minute).Build()
