package vibes

import (
	"testing"
	"time"
)

func TestRecencyWeight(t *testing.T) {
	tests := []struct {
		name string
		age  time.Duration
		want float32
	}{
		{"yesterday", 24 * time.Hour, 1.0},
		{"1 month ago", 30 * 24 * time.Hour, 1.0},
		{"2 months ago", 60 * 24 * time.Hour, 1.0},
		{"4 months ago", 120 * 24 * time.Hour, 0.7},
		{"5 months ago", 150 * 24 * time.Hour, 0.7},
		{"8 months ago", 240 * 24 * time.Hour, 0.4},
		{"11 months ago", 330 * 24 * time.Hour, 0.4},
		{"13 months ago", 395 * 24 * time.Hour, 0.2},
		{"2 years ago", 730 * 24 * time.Hour, 0.2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			showDate := time.Now().Add(-tt.age)
			got := RecencyWeight(showDate)
			if got != tt.want {
				t.Errorf("RecencyWeight(%s ago) = %f, want %f", tt.age, got, tt.want)
			}
		})
	}
}
