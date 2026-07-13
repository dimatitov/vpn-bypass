package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLoadExistingStateFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	data := []byte(`{"gateway":"192.0.2.1","interface":"en0","routes":["198.51.100.1/32"],"updatedAt":"2026-07-13T10:00:00Z"}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	value, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if value.Gateway != "192.0.2.1" || value.Interface != "en0" || !reflect.DeepEqual(value.Routes, []string{"198.51.100.1/32"}) || !value.UpdatedAt.Equal(time.Date(2026, 7, 13, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected state: %+v", value)
	}
}
