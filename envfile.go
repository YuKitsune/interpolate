package main

import (
	"fmt"
	"os"
	"strings"
)

// ReadEnvFile reads the variables from a docker-compose style env file.
// To interpolate a value, it uses lookup first, and then the variables that come before the value in the file.
func ReadEnvFile(path string, lookup Lookup) (map[string]string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	vars, err := parseEnv(string(content), lookup)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return vars, nil
}

func parseEnv(content string, lookup Lookup) (map[string]string, error) {
	vars := map[string]string{}
	lookupAll := func(name string) (string, bool) {
		if value, ok := lookup(name); ok {
			return value, true
		}
		value, ok := vars[name]
		return value, ok
	}

	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i := 0; i < len(lines); i++ {
		lineNumber := i + 1
		line := strings.TrimSpace(lines[i])
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		key, rawValue, hasValue := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !isValidName(key) {
			return nil, fmt.Errorf("line %d: invalid variable name %q", lineNumber, key)
		}

		// A line with only a name copies the value from the environment.
		if !hasValue {
			if value, ok := lookup(key); ok {
				vars[key] = value
			}
			continue
		}

		rawValue = strings.TrimLeft(rawValue, " \t")

		var value string
		var err error
		if rawValue != "" && (rawValue[0] == '\'' || rawValue[0] == '"') {
			quote := rawValue[0]

			// A quoted value can continue on the next lines.
			for findClosingQuote(rawValue, quote) < 0 && i+1 < len(lines) {
				i++
				rawValue += "\n" + lines[i]
			}

			value, err = parseQuotedValue(rawValue, quote, lookupAll)
		} else {
			value, err = parseUnquotedValue(rawValue, lookupAll)
		}

		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNumber, err)
		}

		vars[key] = value
	}

	return vars, nil
}

// parseQuotedValue reads a value in single or double quotes.
// It does not change a value in single quotes.
// It replaces escape sequences and variables in a value in double quotes.
func parseQuotedValue(rawValue string, quote byte, lookup Lookup) (string, error) {
	end := findClosingQuote(rawValue, quote)
	if end < 0 {
		return "", fmt.Errorf("missing closing quote")
	}

	rest := strings.TrimSpace(rawValue[end+1:])
	if rest != "" && !strings.HasPrefix(rest, "#") {
		return "", fmt.Errorf("unexpected text after closing quote: %q", rest)
	}

	value := rawValue[1:end]
	if quote == '\'' {
		return value, nil
	}

	return Interpolate(unescapeDoubleQuoted(value), lookup, ignoreUnset)
}

// parseUnquotedValue removes an inline comment and the surrounding spaces from a value, then replaces the variables.
// An inline comment starts with a "#" that follows a space or a tab.
func parseUnquotedValue(rawValue string, lookup Lookup) (string, error) {
	for i := 1; i < len(rawValue); i++ {
		if rawValue[i] == '#' && (rawValue[i-1] == ' ' || rawValue[i-1] == '\t') {
			rawValue = rawValue[:i]
			break
		}
	}

	return Interpolate(strings.TrimSpace(rawValue), lookup, ignoreUnset)
}

// findClosingQuote returns the index of the quote that closes the value, or -1 if there is no closing quote.
// The first character of value must be the opening quote.
func findClosingQuote(value string, quote byte) int {
	for i := 1; i < len(value); i++ {
		if quote == '"' && value[i] == '\\' {
			i++
			continue
		}
		if value[i] == quote {
			return i
		}
	}
	return -1
}

func unescapeDoubleQuoted(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 == len(value) {
			b.WriteByte(value[i])
			continue
		}

		i++
		switch value[i] {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		case '"', '\\':
			b.WriteByte(value[i])
		default:
			b.WriteByte('\\')
			b.WriteByte(value[i])
		}
	}
	return b.String()
}

func isValidName(name string) bool {
	if name == "" || !isNameStart(name[0]) {
		return false
	}
	for i := 1; i < len(name); i++ {
		if !isNameChar(name[i]) {
			return false
		}
	}
	return true
}

func ignoreUnset(string) {}
