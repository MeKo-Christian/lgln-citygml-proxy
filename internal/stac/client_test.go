package stac_test

import (
	"testing"

	"github.com/meko-tech/lgln-citygml-proxy/internal/stac"
)

func TestParseItemID(t *testing.T) {
	tests := []struct {
		id     string
		wantE  int
		wantN  int
		wantOK bool
	}{
		{"LoD2_32_550_5800_1_ni", 550, 5800, true},
		{"LoD2_32_551_5801_1_ni", 551, 5801, true},
		{"LoD2_32_550_5800_1_ni.gml", 550, 5800, true}, // .gml suffix tolerated
		{"invalid", 0, 0, false},
		{"LoD2_32_abc_5800_1_ni", 0, 0, false},
		{"LoD2_32_550_abc_1_ni", 0, 0, false},
		{"", 0, 0, false},
		{"Foo_32_550_5800_1_ni", 0, 0, false},
		{"LoD2_99_550_5800_1_ni", 0, 0, false},
		{"LoD2_32_-550_5800_1_ni", 0, 0, false},
		{"LoD2_32_550_-5800_1_ni", 0, 0, false},
	}
	for _, tt := range tests {
		e, n, ok := stac.ParseItemID(tt.id)
		if ok != tt.wantOK {
			t.Errorf("ParseItemID(%q) ok=%v, want %v", tt.id, ok, tt.wantOK)
		}
		if ok && (e != tt.wantE || n != tt.wantN) {
			t.Errorf("ParseItemID(%q) = (%d,%d), want (%d,%d)", tt.id, e, n, tt.wantE, tt.wantN)
		}
	}
}
