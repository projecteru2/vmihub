package utils

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRedis(t *testing.T) {
	SetupRedis(nil, t)
	c := GetRedisConn()
	s := MockRedis
	ans := c.Set(context.Background(), "foo", "bar", 0)
	assert.Nil(t, ans.Err())

	// Optionally check values in redis...
	// got, err := s.Get("foo")
	// assert.Nil(t, err)
	// assert.Equal(t, got, "bar")

	// // ... or use a helper for that:
	// s.CheckGet(t, "foo", "bar")
	{
		v, err := c.Get(context.Background(), "foo").Result()
		assert.Nil(t, err)
		assert.Equal(t, "bar", v)

		err = c.Set(context.Background(), "foo1", 1234, 0).Err()
		assert.Nil(t, err)
		v, err = c.Get(context.Background(), "foo1").Result()
		assert.Nil(t, err)
		iv, err := strconv.Atoi(v)
		assert.Nil(t, err)
		assert.Equal(t, 1234, iv)

		err = c.HSet(context.Background(), "foo2", 1, "1", 2, "2").Err()
		assert.Nil(t, err)
		kv, err := c.HGetAll(context.Background(), "foo2").Result()
		assert.Nil(t, err)
		for k, v := range kv {
			assert.Equal(t, k, v)
		}
	}
	{
		hsetAns := c.HSet(context.Background(), "hkey", "info", 1, "slices", "{}", "bool", strconv.FormatBool(true))
		assert.Nil(t, hsetAns.Err())
		hgetAns := c.HGetAll(context.Background(), "hkey")
		assert.Nil(t, hgetAns.Err())
		assert.Len(t, hgetAns.Val(), 3)
		for k, v := range hgetAns.Val() {
			switch k {
			case "info":
				assert.Equal(t, "1", v)
			case "slices":
				assert.Equal(t, "{}", v)
			case "bool":
				assert.Equal(t, "true", v)
				val, err := strconv.ParseBool(v)
				assert.Nil(t, err)
				assert.True(t, val)
			default:
				assert.Failf(t, "invalid key %s", k)
			}
		}
	}

	// TTL and expiration:
	s.Set("foo", "bar")
	s.SetTTL("foo", 10*time.Second)
	s.FastForward(11 * time.Second)
	assert.False(t, s.Exists("foo"))
}
