package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadExistingConfigurationFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	data := []byte(`{"domains":["example.com"],"cidrs":["198.51.100.0/24"]}`)
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	configuration, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(configuration.Domains, []string{"example.com"}) || !reflect.DeepEqual(configuration.CIDRs, []string{"198.51.100.0/24"}) {
		t.Fatalf("unexpected configuration: %+v", configuration)
	}
}
