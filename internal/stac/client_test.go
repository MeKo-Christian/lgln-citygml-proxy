package stac_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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

// stacServer returns a test server that serves a fixed STAC FeatureCollection.
func stacServer(t *testing.T, features []map[string]any) *httptest.Server {
	t.Helper()
	body, _ := json.Marshal(map[string]any{
		"type":     "FeatureCollection",
		"features": features,
	})
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("bbox") == "" {
			t.Error("expected bbox query parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
}

func TestClient_ItemsByBBox(t *testing.T) {
	srv := stacServer(t, []map[string]any{
		{
			"id":   "LoD2_32_550_5800_1_ni",
			"type": "Feature",
			"properties": map[string]any{
				"datetime":    "2023-06-01T00:00:00Z",
				"Aktualitaet": "2023",
			},
		},
		{
			"id":   "LoD2_32_551_5800_1_ni",
			"type": "Feature",
			"properties": map[string]any{
				"datetime": "2022-01-01T00:00:00Z",
			},
		},
	})
	defer srv.Close()

	c := stac.New(srv.URL)
	items, err := c.ItemsByBBox(context.Background(), 9.5, 52.2, 9.8, 52.4)
	if err != nil {
		t.Fatalf("ItemsByBBox: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}
	if items[0].EastingKM != 550 || items[0].NorthingKM != 5800 {
		t.Errorf("item[0] coords = (%d,%d), want (550,5800)", items[0].EastingKM, items[0].NorthingKM)
	}
	// Aktualitaet "2023" takes precedence over datetime → 2023-01-01
	wantTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	if !items[0].UpdatedAt.Equal(wantTime) {
		t.Errorf("item[0].UpdatedAt = %v, want %v", items[0].UpdatedAt, wantTime)
	}
	// No Aktualitaet on item[1] → falls back to datetime "2022-01-01T00:00:00Z"
	wantTime2 := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)
	if !items[1].UpdatedAt.Equal(wantTime2) {
		t.Errorf("item[1].UpdatedAt = %v, want %v", items[1].UpdatedAt, wantTime2)
	}
}

func TestClient_ItemsByBBox_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := stac.New(srv.URL)
	_, err := c.ItemsByBBox(context.Background(), 9.5, 52.2, 9.8, 52.4)
	if err == nil {
		t.Error("expected error for HTTP 500")
	}
}

func TestClient_ItemsByBBox_SkipsUnparsableIDs(t *testing.T) {
	srv := stacServer(t, []map[string]any{
		{"id": "not-a-tile-id", "type": "Feature", "properties": map[string]any{"datetime": "2023-01-01T00:00:00Z"}},
		{"id": "LoD2_32_550_5800_1_ni", "type": "Feature", "properties": map[string]any{"datetime": "2023-01-01T00:00:00Z"}},
	})
	defer srv.Close()

	c := stac.New(srv.URL)
	items, err := c.ItemsByBBox(context.Background(), 9.5, 52.2, 9.8, 52.4)
	if err != nil {
		t.Fatalf("ItemsByBBox: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("got %d items, want 1 (unparsable ID should be skipped)", len(items))
	}
}

func TestClient_ItemsByBBox_Empty(t *testing.T) {
	srv := stacServer(t, []map[string]any{})
	defer srv.Close()

	c := stac.New(srv.URL)
	items, err := c.ItemsByBBox(context.Background(), 9.5, 52.2, 9.8, 52.4)
	if err != nil {
		t.Fatalf("ItemsByBBox: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("got %d items, want 0", len(items))
	}
}
