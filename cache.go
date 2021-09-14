package main

import (
	"flag"
	"time"

	"github.com/bluele/gcache"
)

var cacheTimeout time.Duration
var cacheMax int
var cache gcache.Cache

func init() {
	flag.DurationVar(&cacheTimeout, "cache-timeout", 2*time.Minute, "cache timeout")
	flag.IntVar(&cacheMax, "cache-max", 2000, "cache max")
}
