package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type State struct {
	Gateway   string    `json:"gateway"`
	Interface string    `json:"interface"`
	Routes    []string  `json:"routes"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func Load(path string) (State, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}

	var result State
	if err := json.Unmarshal(data, &result); err != nil {
		return State{}, err
	}
	return result, nil
}

func Save(path string, value State) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0644)
}
