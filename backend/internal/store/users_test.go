package store

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestNewUserStore_NilPool(t *testing.T) {
	_, err := NewUserStore(nil)
	if err == nil {
		t.Fatal("expected error for nil pool")
	}
}

func TestNewUserStore_NonNilPool(t *testing.T) {
	// pgxpool.Pool is safe to reference without connecting; we just need a non-nil pointer.
	s, err := NewUserStore(&pgxpool.Pool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil UserStore")
	}
}
