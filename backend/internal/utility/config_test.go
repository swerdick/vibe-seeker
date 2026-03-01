package utility

import (
	"testing"
)

func TestReadEnvironmentVariable_ReturnsEmptyWhenSetEmpty(t *testing.T) {
	t.Setenv("TEST_EMPTY_VAR", "")

	got := readEnvironmentVariable("TEST_EMPTY_VAR", "fallback")
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestNewConfig_EnvOverrides(t *testing.T) {
	t.Setenv("ENVIRONMENT", "production")
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("PORT", "9090")
	t.Setenv("APP_NAME", "test-app")
	t.Setenv("CORS_ORIGIN", "https://example.com")

	cfg := NewConfig()

	stringTests := []struct {
		name string
		got  string
		want string
	}{
		{"Environment", cfg.Environment, "production"},
		{"LogLevel", cfg.LogLevel, "error"},
		{"AppName", cfg.AppName, "test-app"},
		{"CORSOrigin", cfg.CORSOrigin, "https://example.com"},
	}

	for _, tt := range stringTests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.name, tt.got, tt.want)
		}
	}

	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
}

func TestReadEnvironmentVariable_ReturnsValue(t *testing.T) {
	t.Setenv("TEST_VAR", "hello")

	got := readEnvironmentVariable("TEST_VAR", "fallback")
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestReadEnvironmentVariable_ReturnsDefault(t *testing.T) {
	got := readEnvironmentVariable("TEST_MISSING_VAR_12345", "fallback")
	if got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestReadIntEnvironmentVariable_ReturnsValue(t *testing.T) {
	t.Setenv("TEST_INT_VAR", "3000")

	got := readIntEnvironmentVariable("TEST_INT_VAR", 8080)
	if got != 3000 {
		t.Errorf("got %d, want 3000", got)
	}
}

func TestReadIntEnvironmentVariable_ReturnsDefault(t *testing.T) {
	got := readIntEnvironmentVariable("TEST_MISSING_INT_12345", 8080)
	if got != 8080 {
		t.Errorf("got %d, want 8080", got)
	}
}

func TestReadIntEnvironmentVariable_InvalidReturnsDefault(t *testing.T) {
	t.Setenv("TEST_BAD_INT", "not-a-number")

	got := readIntEnvironmentVariable("TEST_BAD_INT", 8080)
	if got != 8080 {
		t.Errorf("got %d, want 8080 for invalid input", got)
	}
}
