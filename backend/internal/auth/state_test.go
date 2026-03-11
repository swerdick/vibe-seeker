package auth

import (
	"encoding/hex"
	"testing"
)

func TestGenerateState_ReturnsValidHex(t *testing.T) {
	state, err := GenerateState()
	if err != nil {
		t.Fatalf("GenerateState failed: %v", err)
	}

	b, err := hex.DecodeString(state)
	if err != nil {
		t.Fatalf("state is not valid hex: %v", err)
	}

	if len(b) != 16 {
		t.Errorf("expected 16 bytes, got %d", len(b))
	}
}

func TestGenerateState_IsUnique(t *testing.T) {
	states := make(map[string]bool)

	for range 100 {
		state, err := GenerateState()
		if err != nil {
			t.Fatalf("GenerateState failed: %v", err)
		}
		if states[state] {
			t.Fatalf("duplicate state generated: %s", state)
		}
		states[state] = true
	}
}
