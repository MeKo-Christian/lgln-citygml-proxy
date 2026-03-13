package bbox

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    BBox
		wantErr bool
	}{
		{"valid", "549000,5800000,551000,5802000", BBox{549000, 5800000, 551000, 5802000}, false},
		{"too few", "1,2,3", BBox{}, true},
		{"non-numeric", "a,b,c,d", BBox{}, true},
		{"west>east", "551000,5800000,549000,5802000", BBox{}, true},
		{"south>north", "549000,5802000,551000,5800000", BBox{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Parse(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestBBox_TileCoords(t *testing.T) {
	bb := BBox{549000, 5800000, 551999, 5802999}
	coords := bb.TileCoords()

	want := [][2]int{
		{549, 5800},
		{549, 5801},
		{549, 5802},
		{550, 5800},
		{550, 5801},
		{550, 5802},
		{551, 5800},
		{551, 5801},
		{551, 5802},
	}

	if len(coords) != len(want) {
		t.Fatalf("got %d coords, want %d", len(coords), len(want))
	}

	got := make(map[[2]int]bool)
	for _, c := range coords {
		got[c] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("missing tile coord %v", w)
		}
	}
}

func TestBBox_TileCoords_ExactBoundary(t *testing.T) {
	bb := BBox{550000, 5800000, 550999, 5800999}
	coords := bb.TileCoords()
	if len(coords) != 1 {
		t.Fatalf("got %d coords, want 1: %v", len(coords), coords)
	}
	if coords[0] != [2]int{550, 5800} {
		t.Errorf("got %v, want [550 5800]", coords[0])
	}
}
