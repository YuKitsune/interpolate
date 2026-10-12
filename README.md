# interpolate

[![Latest version](https://img.shields.io/github/v/release/YuKitsune/interpolate)](https://github.com/YuKitsune/interpolate/releases/latest)
[![Build](https://img.shields.io/github/actions/workflow/status/YuKitsune/interpolate/release.yml)](https://github.com/YuKitsune/interpolate/actions/workflows/release.yml)

A command-line tool that replaces docker-compose style variables in a file.

## Installation

You must have Go installed.

```sh
go install github.com/yukitsune/interpolate@latest
```

## Usage

```sh
interpolate [flags] [file]
```

The tool reads `file`. If you do not give a file, or if the file is `-`, the tool reads stdin. The tool writes the result to stdout.

| Flag | Description |
| --- | --- |
| `--env <file>` | Read the variables from this file. The default is `.env` in the current directory. |
| `-o`, `--output <file>` | Write the result to this file. |
| `--temp` | Write the result to a temporary file, then write the path of that file to stdout. |

Put the flags before the file.

Variables in the environment have priority over variables in the env file.

### Examples

```sh
interpolate compose.yaml
cat compose.yaml | interpolate
interpolate --env prod.env compose.yaml
interpolate -o out.yaml compose.yaml
docker compose -f "$(interpolate --temp compose.yaml)" up
```

### Supported formats

| Format | Result |
| --- | --- |
| `$VAR`, `${VAR}` | The value of `VAR`. |
| `$$` | A literal `$`. |
| `${VAR:-default}` | `default` if `VAR` is not set or is empty. |
| `${VAR-default}` | `default` if `VAR` is not set. |
| `${VAR:?message}` | An error if `VAR` is not set or is empty. |
| `${VAR?message}` | An error if `VAR` is not set. |
| `${VAR:+replacement}` | `replacement` if `VAR` is set and is not empty. |
| `${VAR+replacement}` | `replacement` if `VAR` is set. |

If a variable is not set and has no default value, the tool uses an empty string and writes a warning to stderr.

## Development

```sh
make build   # Build the binary to bin/interpolate.
make test    # Run the tests.
make clean   # Remove bin/ and the installed binary.
```
