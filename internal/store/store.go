package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrNotFound = errors.New("flag not found")

const flagsSet = "flags"

type Flag struct {
	Name        string    `json:"name"`
	Enabled     bool      `json:"enabled"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Store {
	return &Store{rdb: rdb}
}

func key(name string) string {
	return "flag:" + name
}

func (s *Store) List(ctx context.Context) ([]Flag, error) {
	names, err := s.rdb.SMembers(ctx, flagsSet).Result()
	if err != nil {
		return nil, fmt.Errorf("list flags: %w", err)
	}

	flags := make([]Flag, 0, len(names))
	for _, name := range names {
		f, err := s.Get(ctx, name)
		if err != nil {
			continue
		}
		flags = append(flags, f)
	}
	return flags, nil
}

func (s *Store) Get(ctx context.Context, name string) (Flag, error) {
	vals, err := s.rdb.HGetAll(ctx, key(name)).Result()
	if err != nil {
		return Flag{}, fmt.Errorf("get flag: %w", err)
	}
	if len(vals) == 0 {
		return Flag{}, ErrNotFound
	}
	return unmarshal(name, vals)
}

func (s *Store) Upsert(ctx context.Context, name, description string, enabled bool) (Flag, error) {
	now := time.Now().UTC().Format(time.RFC3339)

	createdAt, err := s.rdb.HGet(ctx, key(name), "created_at").Result()
	if errors.Is(err, redis.Nil) {
		createdAt = now
	} else if err != nil {
		return Flag{}, fmt.Errorf("upsert flag: %w", err)
	}

	pipe := s.rdb.Pipeline()
	pipe.HSet(ctx, key(name),
		"enabled", boolToStr(enabled),
		"description", description,
		"created_at", createdAt,
		"updated_at", now,
	)
	pipe.SAdd(ctx, flagsSet, name)
	if _, err := pipe.Exec(ctx); err != nil {
		return Flag{}, fmt.Errorf("upsert flag: %w", err)
	}

	return s.Get(ctx, name)
}

func (s *Store) Toggle(ctx context.Context, name string) (Flag, error) {
	script := redis.NewScript(`
		local key = KEYS[1]
		local current = redis.call("HGET", key, "enabled")
		if current == false then
			return redis.error_reply("not found")
		end
		local next = current == "true" and "false" or "true"
		redis.call("HSET", key, "enabled", next, "updated_at", ARGV[1])
		return next
	`)

	now := time.Now().UTC().Format(time.RFC3339)
	err := script.Run(ctx, s.rdb, []string{key(name)}, now).Err()
	if err != nil {
		if err.Error() == "not found" {
			return Flag{}, ErrNotFound
		}
		return Flag{}, fmt.Errorf("toggle flag: %w", err)
	}

	return s.Get(ctx, name)
}

func (s *Store) Delete(ctx context.Context, name string) error {
	pipe := s.rdb.Pipeline()
	pipe.Del(ctx, key(name))
	pipe.SRem(ctx, flagsSet, name)
	results, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete flag: %w", err)
	}
	if results[0].(*redis.IntCmd).Val() == 0 {
		return ErrNotFound
	}
	return nil
}

func unmarshal(name string, vals map[string]string) (Flag, error) {
	createdAt, _ := time.Parse(time.RFC3339, vals["created_at"])
	updatedAt, _ := time.Parse(time.RFC3339, vals["updated_at"])
	return Flag{
		Name:        name,
		Enabled:     vals["enabled"] == "true",
		Description: vals["description"],
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
