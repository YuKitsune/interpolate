package main

import (
	"errors"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadInput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.yaml")
	writeFile(t, path, "image: $IMAGE\n")

	got, err := readInput(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "image: $IMAGE\n" {
		t.Errorf("got %q, want %q", got, "image: $IMAGE\n")
	}
}

func TestLoadEnvFile(t *testing.T) {
	t.Run("default file does not exist", func(t *testing.T) {
		t.Chdir(t.TempDir())

		got, err := loadEnvFile("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %q, want no variables", got)
		}
	})

	t.Run("default file exists", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, ".env"), "INTERPOLATE_TEST_VAR=from-default\n")
		t.Chdir(dir)

		got, err := loadEnvFile("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := map[string]string{"INTERPOLATE_TEST_VAR": "from-default"}
		if !maps.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("given file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "custom.env")
		writeFile(t, path, "INTERPOLATE_TEST_VAR=from-custom\n")

		got, err := loadEnvFile(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := map[string]string{"INTERPOLATE_TEST_VAR": "from-custom"}
		if !maps.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("given file does not exist", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.env")

		_, err := loadEnvFile(path)
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("got error %v, want %v", err, fs.ErrNotExist)
		}
	})
}

func TestWriteOutput(t *testing.T) {
	t.Run("stdout", func(t *testing.T) {
		got := captureStdout(t, func() {
			if err := writeOutput("result\n", "", "", false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if got != "result\n" {
			t.Errorf("got %q, want %q", got, "result\n")
		}
	})

	t.Run("output file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "out.yaml")

		stdout := captureStdout(t, func() {
			if err := writeOutput("result\n", "", path, false); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		if stdout != "" {
			t.Errorf("got stdout %q, want no stdout", stdout)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "result\n" {
			t.Errorf("got file content %q, want %q", got, "result\n")
		}
	})

	t.Run("temp file", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir())

		stdout := captureStdout(t, func() {
			if err := writeOutput("result\n", "compose.yaml", "", true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		path := strings.TrimSuffix(stdout, "\n")
		if filepath.Ext(path) != ".yaml" {
			t.Errorf("got temp file %q, want the extension %q", path, ".yaml")
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "result\n" {
			t.Errorf("got file content %q, want %q", got, "result\n")
		}
	})

	t.Run("temp file from stdin has no extension", func(t *testing.T) {
		t.Setenv("TMPDIR", t.TempDir())

		stdout := captureStdout(t, func() {
			if err := writeOutput("result\n", "", "", true); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})

		path := strings.TrimSuffix(stdout, "\n")
		if filepath.Ext(path) != "" {
			t.Errorf("got temp file %q, want no extension", path)
		}
	})
}

// captureStdout runs fn and returns the text that fn writes to stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}

	original := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = original }()
	fn()

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
