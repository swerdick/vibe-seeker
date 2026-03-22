package store

import "testing"

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Big Thief", "big-thief"},
		{"Japanese Breakfast", "japanese-breakfast"},
		{"blink-182", "blink-182"},
		{"AC/DC", "ac-dc"},
		{"Guns N' Roses", "guns-n-roses"},
		{"The War on Drugs", "the-war-on-drugs"},
		{"M83", "m83"},
		{"", ""},
		{"   ", ""},
		{"!!!Panic!!!", "panic"},
		{"Beyoncé", "beyoncé"},
		{"$uicideboy$", "uicideboy"},
		{"100 gecs", "100-gecs"},
		{"A", "a"},
	}

	for _, tt := range tests {
		got := Slugify(tt.input)
		if got != tt.want {
			t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
