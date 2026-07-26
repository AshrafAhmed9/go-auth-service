package main

// testRedisClient gives tests a real Redis to talk to — an in-memory
// server, not a mock of RedisClient — so blacklist behavior is exercised
// through the actual client code, not reimplemented in a fake.
import (
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func testRedisClient(t *testing.T) *RedisClient {
	t.Helper()
	mr := miniredis.RunT(t)
	return newRedisClient(mr.Addr())
}
