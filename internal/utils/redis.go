package utils

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	"github.com/projecteru2/vmihub/config"
	"github.com/redis/go-redis/v9"
)

var (
	cli       *redis.Client
	MockRedis *miniredis.Miniredis
	rs        *redsync.Redsync
)

func NewRedisCient(cfg *config.RedisConfig) (ans *redis.Client) {

	if len(cfg.SentinelAddrs) > 0 {
		ans = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    cfg.MasterName,
			SentinelAddrs: cfg.SentinelAddrs,
			DB:            cfg.DB,
			Username:      cfg.Username,
			Password:      cfg.Password,
		})
	} else {
		ans = redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			DB:       cfg.DB,
			Username: cfg.Username,
			Password: cfg.Password,
		})
	}
	return
}

func SetupRedis(cfg *config.RedisConfig, t *testing.T) {
	if t != nil {
		MockRedis = miniredis.RunT(t)
		cli = redis.NewClient(&redis.Options{
			Addr: MockRedis.Addr(), // Redis 服务器地址
		})
		return
	}
	cli = NewRedisCient(cfg)
	rs = redsync.New(goredis.NewPool(cli))
}

func GetRedisConn() *redis.Client {
	return cli
}

func SetObjToRedis(ctx context.Context, k string, obj any, expiration time.Duration) error {
	bs, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return cli.Set(ctx, k, bs, expiration).Err()
}

func DeleteObjectsInRedis(ctx context.Context, keys ...string) error {
	return cli.Del(ctx, keys...).Err()
}

func GetObjFromRedis(ctx context.Context, k string, obj any) error {
	v, err := cli.Get(ctx, k).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(v), obj)
}

func NewRedisMutex(name string, expiry time.Duration) *redsync.Mutex {
	return rs.NewMutex(name, redsync.WithExpiry(expiry))
}

func CleanRedisMutex(name string) error {
	return cli.Del(context.TODO(), name).Err()
}

func LockRedisKey(ctx context.Context, key string, expiry time.Duration) (func(), error) {
	mtx := NewRedisMutex(key, expiry)
	if err := mtx.LockContext(ctx); err != nil {
		return nil, err
	}
	return func() {
		retryTask := NewRetryTask(ctx, 3, func() error {
			_, err := mtx.Unlock()
			return err
		})
		_ = retryTask.Run(ctx)
	}, nil
}
