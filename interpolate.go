package main

import (
	"fmt"
	"strings"
)

// Lookup returns the value of a variable, and true if the variable is set.
type Lookup func(name string) (string, bool)

// Interpolate replaces the docker-compose style variables in input.
// It supports these formats:
//
//	$$                    a literal "$"
//	$VAR, ${VAR}          the value of VAR
//	${VAR:-default}       default if VAR is not set or is empty
//	${VAR-default}        default if VAR is not set
//	${VAR:?message}       an error if VAR is not set or is empty
//	${VAR?message}        an error if VAR is not set
//	${VAR:+replacement}   replacement if VAR is set and is not empty
//	${VAR+replacement}    replacement if VAR is set
//
// The default, message, and replacement text can contain variables.
// Interpolate calls onUnset for each variable that is not set and has no default value.
func Interpolate(input string, lookup Lookup, onUnset func(name string)) (string, error) {
	p := &parser{input: input, lookup: lookup, onUnset: onUnset}
	return p.parseText(false, true)
}

type parser struct {
	input   string
	pos     int
	lookup  Lookup
	onUnset func(name string)
}

// parseText reads text until the end of the input.
// If insideBraces is true, it stops before the closing brace.
// If eval is false, it only reads the text. It does not look up variables or return errors for missing values.
func (p *parser) parseText(insideBraces bool, eval bool) (string, error) {
	var b strings.Builder
	for p.pos < len(p.input) {
		c := p.input[p.pos]
		if insideBraces && c == '}' {
			break
		}

		if c != '$' {
			b.WriteByte(c)
			p.pos++
			continue
		}

		value, err := p.parseDollar(eval)
		if err != nil {
			return "", err
		}
		b.WriteString(value)
	}

	return b.String(), nil
}

func (p *parser) parseDollar(eval bool) (string, error) {
	start := p.pos
	p.pos++ // Skip the "$".
	if p.pos >= len(p.input) {
		return "$", nil
	}

	c := p.input[p.pos]
	switch {
	case c == '$':
		p.pos++
		return "$", nil
	case c == '{':
		p.pos++
		return p.parseBraced(start, eval)
	case isNameStart(c):
		name := p.readName()
		if !eval {
			return "", nil
		}
		return p.resolve(name), nil
	default:
		return "$", nil
	}
}

func (p *parser) parseBraced(start int, eval bool) (string, error) {
	name := p.readName()
	if name == "" {
		return "", p.invalid(start, "missing variable name")
	}

	if p.pos >= len(p.input) {
		return "", p.invalid(start, "missing closing brace")
	}

	if p.input[p.pos] == '}' {
		p.pos++
		if !eval {
			return "", nil
		}
		return p.resolve(name), nil
	}

	colon := false
	if p.input[p.pos] == ':' {
		colon = true
		p.pos++
	}

	if p.pos >= len(p.input) {
		return "", p.invalid(start, "missing closing brace")
	}

	op := p.input[p.pos]
	if op != '-' && op != '?' && op != '+' {
		return "", p.invalid(start, fmt.Sprintf("unexpected character %q after variable name", op))
	}
	p.pos++

	value, set := "", false
	if eval {
		value, set = p.lookup(name)
	}
	missing := !set || (colon && value == "")

	var useArg bool
	switch op {
	case '-', '?':
		useArg = missing
	case '+':
		useArg = !missing
	}

	arg, err := p.parseText(true, eval && useArg)
	if err != nil {
		return "", err
	}

	if p.pos >= len(p.input) {
		return "", p.invalid(start, "missing closing brace")
	}
	p.pos++ // Skip the "}".

	if !eval {
		return "", nil
	}

	switch op {
	case '-':
		if missing {
			return arg, nil
		}
		return value, nil
	case '?':
		if missing {
			if arg == "" {
				return "", fmt.Errorf("required variable %s is missing a value", name)
			}
			return "", fmt.Errorf("required variable %s is missing a value: %s", name, arg)
		}
		return value, nil
	default: // '+'
		if missing {
			return "", nil
		}
		return arg, nil
	}
}

func (p *parser) resolve(name string) string {
	value, ok := p.lookup(name)
	if !ok {
		p.onUnset(name)
	}
	return value
}

func (p *parser) readName() string {
	start := p.pos
	if p.pos < len(p.input) && isNameStart(p.input[p.pos]) {
		p.pos++
		for p.pos < len(p.input) && isNameChar(p.input[p.pos]) {
			p.pos++
		}
	}
	return p.input[start:p.pos]
}

func (p *parser) invalid(start int, reason string) error {
	line := strings.Count(p.input[:start], "\n") + 1
	return fmt.Errorf("invalid interpolation format on line %d: %s", line, reason)
}

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isNameChar(c byte) bool {
	return isNameStart(c) || (c >= '0' && c <= '9')
}
