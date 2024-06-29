package redissession

import (
	"context"
	"errors"

	ginsessions "github.com/gin-contrib/sessions"
	"github.com/rbcervilla/redisstore/v9"
	"github.com/redis/go-redis/v9"
)

type Store interface {
	ginsessions.Store
}

// NewStore - create new session store with given redis client interface
func NewStore(ctx context.Context, client redis.UniversalClient) (ginsessions.Store, error) {
	innerStore, err := redisstore.NewRedisStore(ctx, client)
	if err != nil {
		return nil, err
	}
	return &store{innerStore}, nil
}

type store struct {
	*redisstore.RedisStore
}

// GetRedisStore get the actual woking store.
// Ref: https://godoc.org/github.com/boj/redistore#RediStore
func GetRedisStore(s Store) (rediStore *redisstore.RedisStore, err error) {
	realStore, ok := s.(*store)
	if !ok {
		err = errors.New("unable to get the redis store: Store isn't *store")
		return
	}

	rediStore = realStore.RedisStore
	return
}

// SetKeyPrefix sets the key prefix in the redis database.
func SetKeyPrefix(s Store, prefix string) error {
	rediStore, err := GetRedisStore(s)
	if err != nil {
		return err
	}

	rediStore.KeyPrefix(prefix)
	return nil
}

func (c *store) Options(options ginsessions.Options) {
	c.RedisStore.Options(*options.ToGorillaOptions())
}
