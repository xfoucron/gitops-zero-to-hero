package store

import (
	"backend/internal/models"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	clickStreamName = "link_clicks"
	cacheTTL        = 1 * time.Hour
)

type RedisStore struct {
	client *redis.Client
}

func NewRedisStore(url string) (*RedisStore, error) {
	opt, err := redis.ParseURL(url)

	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisStore{client: client}, nil
}

func (r *RedisStore) Close() error {
	return r.client.Close()
}

func (r *RedisStore) CacheTarget(ctx context.Context, linkId, targetURL string) error {
	return r.client.Set(ctx, "link:"+linkId, targetURL, cacheTTL).Err()
}

func (r *RedisStore) GetCachedTarget(ctx context.Context, linkId string) (string, error) {
	val, err := r.client.Get(ctx, "link:"+linkId).Result()

	if errors.Is(err, redis.Nil) {
		return "", nil
	}

	return val, err
}

func (r *RedisStore) PublishClick(ctx context.Context, event models.ClickEvent) error {
	payload, err := json.Marshal(event)

	if err != nil {
		return err
	}

	return r.client.XAdd(ctx, &redis.XAddArgs{
		Stream: clickStreamName,
		Values: map[string]interface{}{"data": payload}},
	).Err()
}
