# AGENTS.md

## Project Overview

`gopass-jsonapi` is a Go-based command-line application that enables communication with `gopass` via JSON messages. It is primarily used as a native messaging host for browser plugins like `gopassbridge`, allowing them to interact with a user's `gopass` store. This project acts as an integration or a wrapper around the main `gopass` application logic, exposing it through a simple JSON API over stdin/stdout.

## Folder Structure

- `main.go`: The entry point for the application. It defines the command-line interface (CLI) using the `urfave/cli` library. The main commands are `listen` (to start the JSON API) and `configure` (to set up browser integration).
- `jsonapi.go`: Contains the Go implementation of the JSON API service, including the `listen` and `setup` functions called by the CLI commands.
- `internal/jsonapi/`: Contains the core logic for the JSON API.
  - `api.go`: Defines the main API structure and the message-serving loop.
  - `messages.go`: Defines the structure of incoming JSON requests from clients.
  - `responses.go`: Defines the structure of outgoing JSON responses.
  - `manifest/`: Contains logic for creating and installing the native messaging manifests required by browsers.
- `docs/api.md`: The official documentation for the JSON API, detailing the request and response formats for different actions like `query`, `getLogin`, etc.
- `Makefile`: Contains various targets for building, testing, formatting, and linting the code.
- `go.mod`: The Go module file, which lists the project's dependencies, most notably `github.com/gopasspw/gopass`.
- `gopass_wrapper.sh`: A simple shell script that is installed as part of the browser configuration to act as the native messaging host.

## Key Commands

The following commands are defined in the `Makefile` and are essential for development:

- `make build`: Compiles the Go source code and creates the `gopass-jsonapi` binary.
- `make test`: Runs the unit tests for the project.
- `make fmt`: Formats the Go source code according to the project's style guidelines using `gofumpt` and `gci`.
- `make codequality`: Runs `golangci-lint` to perform static analysis and check for code quality issues.

## Libraries and Frameworks

- **[github.com/gopasspw/gopass](https://github.com/gopasspw/gopass)**: The core `gopass` library. `gopass-jsonapi` uses this as a dependency to interact with password stores.
- **[github.com/urfave/cli/v2](https://github.com/urfave/cli)**: A library for building command-line applications in Go. It is used to structure the `listen` and `configure` commands.

## Testing instructions

- Before submitting any changes, please ensure that the code is correctly formatted by running `make fmt`.
- Always run the test suite with `make test` to ensure that your changes have not broken any existing functionality.
- Run `make codequality` to check for any linting or static analysis issues.
- All checks should pass before a pull request is considered for merging.
