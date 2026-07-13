package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

type Config struct {
	Domains []string `json:"domains"`
	CIDRs   []string `json:"cidrs"`
}

type PathSet struct {
	Directory string
	Config    string
	State     string
	Logs      string
}

func Paths() (PathSet, error) {
	var dir, logs string

	switch runtime.GOOS {
	case "darwin":
		dir = "/Library/Application Support/vpn-bypass"
		logs = "/Library/Logs/vpn-bypass"
	case "windows":
		base := os.Getenv("ProgramData")
		if base == "" {
			return PathSet{}, fmt.Errorf("ProgramData is not set")
		}
		dir = filepath.Join(base, "vpn-bypass")
		logs = filepath.Join(dir, "logs")
	default:
		return PathSet{}, fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	return PathSet{
		Directory: dir,
		Config:    filepath.Join(dir, "config.json"),
		State:     filepath.Join(dir, "state.json"),
		Logs:      logs,
	}, nil
}

func Ensure() (PathSet, error) {
	paths, err := Paths()
	if err != nil {
		return PathSet{}, err
	}
	return ensure(paths)
}

func ensure(paths PathSet) (PathSet, error) {
	if _, err := Load(paths.Config); err != nil {
		return PathSet{}, err
	}
	if err := os.MkdirAll(paths.Logs, 0755); err != nil {
		return PathSet{}, fmt.Errorf("create log directory: %w", err)
	}
	return paths, nil
}

func Purge() error {
	paths, err := Paths()
	if err != nil {
		return err
	}
	return purge(paths)
}

func purge(paths PathSet) error {
	var errs []error
	if err := os.RemoveAll(paths.Directory); err != nil {
		errs = append(errs, fmt.Errorf("remove application data: %w", err))
	}
	if paths.Logs != paths.Directory {
		if err := os.RemoveAll(paths.Logs); err != nil {
			errs = append(errs, fmt.Errorf("remove logs: %w", err))
		}
	}
	return errors.Join(errs...)
}

func Default() Config {
	return Config{
		Domains: []string{
			"ozon.ru",
			"www.ozon.ru",
			"api.ozon.ru",
			"ir.ozon.ru",
			"cdn1.ozone.ru",
			"cdn2.ozone.ru",
			"cdn.ozone.ru",
			"yandex.ru",
			"www.yandex.ru",
			"ya.ru",
			"yastatic.net",
			"yandex.net",
			"yandex.com",
			"passport.yandex.ru",
			"mail.yandex.ru",
			"market.yandex.ru",
			"disk.yandex.ru",
			"music.yandex.ru",
			"avito.ru",
			"www.avito.ru",
			"m.avito.ru",
			"img.avito.st",
			"static.avito.ru",
			"avito.st",
			"gosuslugi.ru",
			"www.gosuslugi.ru",
			"esia.gosuslugi.ru",
			"pos.gosuslugi.ru",
			"lk.gosuslugi.ru",
		},
		CIDRs: []string{},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := Default()
		if err := Save(path, cfg); err != nil {
			return Config{}, err
		}
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("read config.json: %w", err)
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	sort.Strings(cfg.Domains)
	sort.Strings(cfg.CIDRs)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0644)
}
