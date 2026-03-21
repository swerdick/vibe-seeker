package store

import "testing"

func TestNewUserStore_NotNil(t *testing.T) {
	s := NewUserStore(nil)
	if s == nil {
		t.Fatal("expected non-nil UserStore")
	}
}
