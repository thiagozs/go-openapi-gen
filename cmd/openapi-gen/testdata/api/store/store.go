package store

import (
	"context"
	"errors"
	"time"
)

type Alias struct {
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	URLKey    string    `json:"url_key"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Policy struct {
	Keep int
	TTL  time.Duration
}

type Store struct{}

var errNotFound = errors.New("not found")

func IsNotFound(err error) bool { return errors.Is(err, errNotFound) }

func (Store) UpsertAlias(ctx context.Context, name, url, key string) error { return nil }
func (Store) GetAlias(ctx context.Context, name string) (Alias, error)     { return Alias{}, nil }
func (Store) DeleteAlias(ctx context.Context, name string) error           { return nil }
func (Store) GetRetentionPolicy(ctx context.Context) (Policy, error)       { return Policy{}, nil }
func (Store) SetRetentionPolicy(ctx context.Context, p Policy) error       { return nil }
