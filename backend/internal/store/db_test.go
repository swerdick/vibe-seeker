package store

import (
	"context"
	"testing"
)

func TestConnect_InvalidURL(t *testing.T) {
	_, err := Connect(context.Background(), "not-a-valid-url://bad")
	if err == nil {
		t.Fatal("expected error for invalid database URL")
	}
}
