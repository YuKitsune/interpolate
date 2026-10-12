package main

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEnv(t *testing.T) {
	tests := []struct {
		name    string
		content string
		env     map[string]string
		want    map[string]string
	}{
		{name: "empty file", content: "", want: map[string]string{}},
		{name: "one variable", content: "FOO=bar", want: map[string]string{"FOO": "bar"}},
		{name: "many variables", content: "FOO=bar\nBAZ=qux\n", want: map[string]string{"FOO": "bar", "BAZ": "qux"}},
		{name: "comments and blank lines", content: "# comment\n\nFOO=bar\n  # indented comment\n", want: map[string]string{"FOO": "bar"}},
		{name: "export prefix", content: "export FOO=bar", want: map[string]string{"FOO": "bar"}},
		{name: "spaces around name and value", content: "  FOO = bar  ", want: map[string]string{"FOO": "bar"}},
		{name: "empty value", content: "FOO=", want: map[string]string{"FOO": ""}},
		{name: "value contains equals sign", content: "FOO=a=b", want: map[string]string{"FOO": "a=b"}},
		{name: "windows line endings", content: "FOO=bar\r\nBAZ=qux\r\n", want: map[string]string{"FOO": "bar", "BAZ": "qux"}},
		{name: "later value replaces earlier value", content: "FOO=a\nFOO=b", want: map[string]string{"FOO": "b"}},

		{name: "inline comment after space", content: "FOO=bar # comment", want: map[string]string{"FOO": "bar"}},
		{name: "inline comment after tab", content: "FOO=bar\t# comment", want: map[string]string{"FOO": "bar"}},
		{name: "hash without space is part of value", content: "FOO=bar#baz", want: map[string]string{"FOO": "bar#baz"}},

		{name: "single quotes keep value as written", content: `FOO='a $BAR \n # b'`, want: map[string]string{"FOO": `a $BAR \n # b`}},
		{name: "single quotes with comment after", content: `FOO='bar' # comment`, want: map[string]string{"FOO": "bar"}},
		{name: "single quotes on many lines", content: "FOO='line1\nline2'\nBAR=x", want: map[string]string{"FOO": "line1\nline2", "BAR": "x"}},

		{name: "double quotes with escapes", content: `FOO="a\nb\r\t\"c\"\\"`, want: map[string]string{"FOO": "a\nb\r\t\"c\"\\"}},
		{name: "double quotes keep unknown escapes", content: `FOO="a\xb"`, want: map[string]string{"FOO": `a\xb`}},
		{name: "double quotes keep hash", content: `FOO="a # b"`, want: map[string]string{"FOO": "a # b"}},
		{name: "double quotes keep spaces", content: `FOO="  a  "`, want: map[string]string{"FOO": "  a  "}},
		{name: "double quotes with comment after", content: `FOO="bar" # comment`, want: map[string]string{"FOO": "bar"}},
		{name: "double quotes on many lines", content: "FOO=\"line1\nline2\"\nBAR=x", want: map[string]string{"FOO": "line1\nline2", "BAR": "x"}},

		{name: "unquoted value uses earlier variable", content: "A=1\nB=${A}2", want: map[string]string{"A": "1", "B": "12"}},
		{name: "double quoted value uses earlier variable", content: "A=1\nB=\"$A\"", want: map[string]string{"A": "1", "B": "1"}},
		{name: "unquoted escaped dollar", content: "FOO=a$$b", want: map[string]string{"FOO": "a$b"}},
		{name: "unset variable becomes empty", content: "FOO=$MISSING", want: map[string]string{"FOO": ""}},
		{name: "environment has priority in interpolation", content: "A=file\nB=$A", env: map[string]string{"A": "env"}, want: map[string]string{"A": "file", "B": "env"}},
		{name: "value uses environment", content: "FOO=${HOME_DIR}/x", env: map[string]string{"HOME_DIR": "/home"}, want: map[string]string{"FOO": "/home/x"}},

		{name: "name only copies value from environment", content: "FOO", env: map[string]string{"FOO": "x"}, want: map[string]string{"FOO": "x"}},
		{name: "name only without environment value is ignored", content: "FOO", want: map[string]string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseEnv(tt.content, mapLookup(tt.env))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !maps.Equal(got, tt.want) {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseEnvErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{name: "name starts with digit", content: "1FOO=bar", wantErr: `line 1: invalid variable name "1FOO"`},
		{name: "name contains dash", content: "FOO-BAR=x", wantErr: `invalid variable name "FOO-BAR"`},
		{name: "empty name", content: "=bar", wantErr: `invalid variable name ""`},
		{name: "missing closing double quote", content: `FOO="bar`, wantErr: "missing closing quote"},
		{name: "missing closing single quote", content: `FOO='bar`, wantErr: "missing closing quote"},
		{name: "text after closing quote", content: `FOO="bar" baz`, wantErr: `unexpected text after closing quote: "baz"`},
		{name: "interpolation error", content: "FOO=${BAR:?need bar}", wantErr: "required variable BAR is missing a value: need bar"},
		{name: "line number", content: "A=1\n\nB='x", wantErr: "line 3:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseEnv(tt.content, mapLookup(nil))
			if err == nil {
				t.Fatalf("expected an error that contains %q, got no error", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %q, want an error that contains %q", err, tt.wantErr)
			}
		})
	}
}

func TestReadEnvFile(t *testing.T) {
	t.Run("reads file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "test.env")
		writeFile(t, path, "FOO=bar\n")

		got, err := ReadEnvFile(path, mapLookup(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := map[string]string{"FOO": "bar"}
		if !maps.Equal(got, want) {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.env")

		_, err := ReadEnvFile(path, mapLookup(nil))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("got error %v, want %v", err, fs.ErrNotExist)
		}
	})

	t.Run("parse error contains path", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "bad.env")
		writeFile(t, path, "1FOO=bar\n")

		_, err := ReadEnvFile(path, mapLookup(nil))
		if err == nil {
			t.Fatal("expected an error, got no error")
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("got error %q, want an error that contains %q", err, path)
		}
	})
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
