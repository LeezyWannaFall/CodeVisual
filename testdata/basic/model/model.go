// Package model describes the core domain.
package model

// Store holds entities.
type Store[T any] struct {
 Items []T
}

// Reader supplies a value.
type Reader interface {
 Read() string
}

type Alias = string
type Identifier string

// Read returns a placeholder.
func (s *Store[T]) Read() string { return "ok" }
func (s Store[T]) Count() int { return len(s.Items) }
func New[T any]() *Store[T] { return &Store[T]{} }
func hidden() {}
func init() {}
func init() {}
