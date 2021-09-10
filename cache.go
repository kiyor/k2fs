package main

import (
	"time"

	"github.com/bluele/gcache"
)

var cache = gcache.New(2000).LRU().Expiration(2 * time.Minute).Build()
