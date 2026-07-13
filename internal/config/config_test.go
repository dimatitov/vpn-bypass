package config

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestDefaultConfigurationHasExactDomainSet(t *testing.T) {
	want := []string{
		"api.ozon.ru",
		"avito.ru",
		"avito.st",
		"cdn.ozone.ru",
		"cdn1.ozone.ru",
		"cdn2.ozone.ru",
		"disk.yandex.ru",
		"esia.gosuslugi.ru",
		"gosuslugi.ru",
		"lk.gosuslugi.ru",
		"m.avito.ru",
		"mail.yandex.ru",
		"market.yandex.ru",
		"music.yandex.ru",
		"ozon.ru",
		"passport.yandex.ru",
		"pos.gosuslugi.ru",
		"static.avito.ru",
		"www.avito.ru",
		"www.gosuslugi.ru",
		"www.ozon.ru",
		"www.yandex.ru",
		"ya.ru",
		"yandex.com",
		"yandex.net",
		"yandex.ru",
		"yastatic.net",
	}
	got := append([]string(nil), Default().Domains...)
	sort.Strings(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected default domain set:\n got: %v\nwant: %v", got, want)
	}
}

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

func TestEnsurePreservesExistingConfiguration(t *testing.T) {
	directory := t.TempDir()
	paths := PathSet{
		Directory: directory,
		Config:    filepath.Join(directory, "config.json"),
		State:     filepath.Join(directory, "state.json"),
		Logs:      filepath.Join(directory, "logs"),
	}
	original := []byte("{\n  \"domains\": [\"custom.example\"],\n  \"cidrs\": []\n}\n")
	if err := os.WriteFile(paths.Config, original, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ensure(paths); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("existing configuration changed:\n%s", after)
	}
	if info, err := os.Stat(paths.Logs); err != nil || !info.IsDir() {
		t.Fatalf("log directory was not created: %v", err)
	}
}

func TestEnsureCreatesDefaultConfiguration(t *testing.T) {
	directory := t.TempDir()
	paths := PathSet{
		Directory: directory,
		Config:    filepath.Join(directory, "config.json"),
		State:     filepath.Join(directory, "state.json"),
		Logs:      filepath.Join(directory, "logs"),
	}
	if _, err := ensure(paths); err != nil {
		t.Fatal(err)
	}
	configuration, err := Load(paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	expected := Default()
	sort.Strings(expected.Domains)
	sort.Strings(expected.CIDRs)
	if !reflect.DeepEqual(configuration, expected) {
		t.Fatalf("unexpected fresh configuration: %+v", configuration)
	}
}

func TestPurgeRemovesDataAndExternalLogs(t *testing.T) {
	root := t.TempDir()
	paths := PathSet{
		Directory: filepath.Join(root, "data"),
		Config:    filepath.Join(root, "data", "config.json"),
		State:     filepath.Join(root, "data", "state.json"),
		Logs:      filepath.Join(root, "logs"),
	}
	if err := os.MkdirAll(paths.Directory, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(paths.Logs, 0755); err != nil {
		t.Fatal(err)
	}
	if err := purge(paths); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.Directory, paths.Logs} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("path was not purged: %s", path)
		}
	}
}
