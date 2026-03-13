package stac

import (
	"testing"
	"time"
)

func TestParseTime(t *testing.T) {
	tests := []struct {
		s    string
		want time.Time
		ok   bool
	}{
		{"", time.Time{}, false},
		{"2023-06-01T00:00:00Z", time.Date(2023, 6, 1, 0, 0, 0, 0, time.UTC), true},
		{"2022-09-15", time.Date(2022, 9, 15, 0, 0, 0, 0, time.UTC), true},
		{"2022", time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC), true},
		{"notadate", time.Time{}, false},
	}
	for _, tt := range tests {
		got, ok := parseTime(tt.s)
		if ok != tt.ok {
			t.Errorf("parseTime(%q) ok=%v, want %v", tt.s, ok, tt.ok)
		}
		if ok && !got.Equal(tt.want) {
			t.Errorf("parseTime(%q) = %v, want %v", tt.s, got, tt.want)
		}
	}
}
