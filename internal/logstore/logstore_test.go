package logstore

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestOpenAndShow(t *testing.T) {
	directory := t.TempDir()
	file, err := Open(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("hello\n"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Show(context.Background(), Path(directory), &out, false); err != nil {
		t.Fatal(err)
	}
	if out.String() != "hello\n" {
		t.Fatalf("unexpected logs: %q", out.String())
	}
}

func TestShowMissingFile(t *testing.T) {
	err := Show(context.Background(), Path(t.TempDir()), &bytes.Buffer{}, false)
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error: %v", err)
	}
}
