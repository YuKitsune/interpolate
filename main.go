package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

const defaultEnvFile = ".env"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "interpolate: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var envPath, outputPath string
	var useTemp bool
	flag.StringVar(&envPath, "env", "", "read the variables from this `file` (default \".env\" in the current directory)")
	flag.StringVar(&outputPath, "output", "", "write the result to this `file`")
	flag.StringVar(&outputPath, "o", "", "short for --output")
	flag.BoolVar(&useTemp, "temp", false, "write the result to a temporary file, then write the path of that file to stdout")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: interpolate [flags] [file]\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Replaces docker-compose style variables in file, or in stdin if there is no file or file is \"-\".\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Variables in the environment have priority over variables in the env file.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Flags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if outputPath != "" && useTemp {
		return errors.New("you cannot use --output and --temp together")
	}

	if flag.NArg() > 1 {
		return fmt.Errorf("too many arguments: expected 1 file, got %d (put the flags before the file)", flag.NArg())
	}
	inputPath := flag.Arg(0)

	input, err := readInput(inputPath)
	if err != nil {
		return err
	}

	envVars, err := loadEnvFile(envPath)
	if err != nil {
		return err
	}

	lookup := func(name string) (string, bool) {
		if value, ok := os.LookupEnv(name); ok {
			return value, true
		}
		value, ok := envVars[name]
		return value, ok
	}

	warned := map[string]bool{}
	warnUnset := func(name string) {
		if warned[name] {
			return
		}
		warned[name] = true
		fmt.Fprintf(os.Stderr, "interpolate: warning: the %q variable is not set, so an empty string is used\n", name)
	}

	result, err := Interpolate(string(input), lookup, warnUnset)
	if err != nil {
		return err
	}

	return writeOutput(result, inputPath, outputPath, useTemp)
}

func readInput(path string) ([]byte, error) {
	if path != "" && path != "-" {
		return os.ReadFile(path)
	}

	info, err := os.Stdin.Stat()
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeCharDevice != 0 {
		return nil, errors.New("no input: give a file, or send the content to stdin")
	}

	return io.ReadAll(os.Stdin)
}

// loadEnvFile reads the variables from the env file.
// If the user did not give an env file, and the default env file does not exist, it returns no variables.
func loadEnvFile(path string) (map[string]string, error) {
	if path != "" {
		return ReadEnvFile(path, os.LookupEnv)
	}

	vars, err := ReadEnvFile(defaultEnvFile, os.LookupEnv)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	return vars, err
}

func writeOutput(result string, inputPath string, outputPath string, useTemp bool) error {
	switch {
	case outputPath != "":
		return os.WriteFile(outputPath, []byte(result), 0o644)

	case useTemp:
		// Keep the extension of the input file, so that other tools can identify the file type.
		pattern := "interpolate-*"
		if inputPath != "" && inputPath != "-" {
			pattern += filepath.Ext(inputPath)
		}

		file, err := os.CreateTemp("", pattern)
		if err != nil {
			return err
		}

		if _, err := file.WriteString(result); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}

		_, err = fmt.Println(file.Name())
		return err

	default:
		_, err := io.WriteString(os.Stdout, result)
		return err
	}
}
