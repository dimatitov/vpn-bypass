package logstore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const FileName = "vpn-bypass.log"

func Path(directory string) string {
	return filepath.Join(directory, FileName)
}

func Open(directory string) (*os.File, error) {
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}
	file, err := os.OpenFile(Path(directory), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return file, nil
}

func Show(ctx context.Context, path string, out io.Writer, follow bool) error {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("log file does not exist: %s", path)
		}
		return fmt.Errorf("open log file: %w", err)
	}
	defer file.Close()

	for {
		if _, err := io.Copy(out, file); err != nil {
			return fmt.Errorf("read log file: %w", err)
		}
		if !follow {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}
