package main

import (
	"slices"
	"strings"
	"testing"
)

func mapLookup(vars map[string]string) Lookup {
	return func(name string) (string, bool) {
		value, ok := vars[name]
		return value, ok
	}
}

var testVars = map[string]string{
	"FOO":   "foo",
	"BAR":   "bar",
	"EMPTY": "",
}

func TestInterpolate(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      string
		wantUnset []string
	}{
		{name: "plain text", input: "hello world", want: "hello world"},
		{name: "empty input", input: "", want: ""},
		{name: "named variable", input: "a $FOO b", want: "a foo b"},
		{name: "braced variable", input: "a ${FOO} b", want: "a foo b"},
		{name: "braced variable followed by text", input: "${FOO}bar", want: "foobar"},
		{name: "named variable includes following name characters", input: "$FOObar", want: "", wantUnset: []string{"FOObar"}},
		{name: "two variables", input: "$FOO-$BAR", want: "foo-bar"},
		{name: "escaped dollar", input: "$$", want: "$"},
		{name: "escaped dollar before name", input: "$$FOO", want: "$FOO"},
		{name: "dollar at end", input: "cost $", want: "cost $"},
		{name: "dollar before digit", input: "$1", want: "$1"},
		{name: "dollar before symbol", input: "$-", want: "$-"},
		{name: "unset named variable", input: "a${MISSING}b", want: "ab", wantUnset: []string{"MISSING"}},
		{name: "unset variable is reported each time", input: "$MISSING $MISSING", want: " ", wantUnset: []string{"MISSING", "MISSING"}},
		{name: "multi-line input", input: "a: $FOO\nb: ${BAR}\n", want: "a: foo\nb: bar\n"},

		{name: "colon default with set variable", input: "${FOO:-def}", want: "foo"},
		{name: "colon default with empty variable", input: "${EMPTY:-def}", want: "def"},
		{name: "colon default with unset variable", input: "${MISSING:-def}", want: "def"},
		{name: "default with set variable", input: "${FOO-def}", want: "foo"},
		{name: "default with empty variable", input: "${EMPTY-def}", want: ""},
		{name: "default with unset variable", input: "${MISSING-def}", want: "def"},
		{name: "empty default", input: "${MISSING:-}", want: ""},
		{name: "default with spaces", input: "${MISSING:-hello world}", want: "hello world"},

		{name: "colon replacement with set variable", input: "${FOO:+rep}", want: "rep"},
		{name: "colon replacement with empty variable", input: "${EMPTY:+rep}", want: ""},
		{name: "colon replacement with unset variable", input: "${MISSING:+rep}", want: ""},
		{name: "replacement with set variable", input: "${FOO+rep}", want: "rep"},
		{name: "replacement with empty variable", input: "${EMPTY+rep}", want: "rep"},
		{name: "replacement with unset variable", input: "${MISSING+rep}", want: ""},

		{name: "colon required with set variable", input: "${FOO:?err}", want: "foo"},
		{name: "required with empty variable", input: "${EMPTY?err}", want: ""},

		{name: "nested default", input: "${MISSING:-${FOO}}", want: "foo"},
		{name: "nested default in default", input: "${MISSING:-${OTHER:-x}}", want: "x"},
		{name: "nested replacement", input: "${FOO:+[$BAR]}", want: "[bar]"},
		{name: "unused default is not evaluated", input: "${FOO:-${MISSING:?boom}}", want: "foo"},
		{name: "unused default does not report unset variables", input: "${FOO:-$MISSING}", want: "foo"},
		{name: "unused replacement does not report unset variables", input: "${MISSING:+$OTHER}", want: ""},
		{name: "escaped dollar in default", input: "${MISSING:-$$FOO}", want: "$FOO"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var unset []string
			got, err := Interpolate(tt.input, mapLookup(testVars), func(name string) {
				unset = append(unset, name)
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
			if !slices.Equal(unset, tt.wantUnset) {
				t.Errorf("got unset variables %q, want %q", unset, tt.wantUnset)
			}
		})
	}
}

func TestInterpolateErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{name: "colon required with unset variable", input: "${MISSING:?custom message}", wantErr: "required variable MISSING is missing a value: custom message"},
		{name: "colon required with empty variable", input: "${EMPTY:?}", wantErr: "required variable EMPTY is missing a value"},
		{name: "required with unset variable", input: "${MISSING?}", wantErr: "required variable MISSING is missing a value"},
		{name: "required message contains variables", input: "${MISSING:?need $FOO}", wantErr: "required variable MISSING is missing a value: need foo"},
		{name: "nested required in used default", input: "${MISSING:-${OTHER:?boom}}", wantErr: "required variable OTHER is missing a value: boom"},
		{name: "empty braces", input: "${}", wantErr: "missing variable name"},
		{name: "name starts with digit", input: "${1}", wantErr: "missing variable name"},
		{name: "unclosed brace", input: "${FOO", wantErr: "missing closing brace"},
		{name: "unclosed brace after colon", input: "${FOO:", wantErr: "missing closing brace"},
		{name: "unclosed brace after default", input: "${FOO:-abc", wantErr: "missing closing brace"},
		{name: "unexpected character", input: "${FOO!}", wantErr: "unexpected character '!' after variable name"},
		{name: "unexpected character after colon", input: "${FOO:x}", wantErr: "unexpected character 'x' after variable name"},
		{name: "line number", input: "a\nb\n${}", wantErr: "on line 3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Interpolate(tt.input, mapLookup(testVars), func(string) {})
			if err == nil {
				t.Fatalf("expected an error that contains %q, got no error", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got error %q, want an error that contains %q", err, tt.wantErr)
			}
		})
	}
}
