# Nagini
## Installing
1. Install Go
2. Run:
```bash
go install .
```
3. Installs from `$GOPATH/bin/nagini`
## Usage
### Note: Run from `$GOPATH/bin/nagini`, best option is to add this to your PATH. Another option is to run from raw sources files as specified in the "Contributing" section.
- Help
```bash
nagini --help
```
- Log Pulling
```bash
nagini log [config YAML] [flags]
```
## Examples
_TODO_
## Running from Repo
1. Clone repo
2. Run `--help` flag to see available options. (_TODO: Add better info about command options_)
```bash
go run . --help
```
## Contributing
### Running locally from source files
1. Clone repo
2. Run `--help` flag to see available options. (_TODO: Add better info about command options_)
```bash
go run . --help
```
### Running Tests
1. Clone repo & Open shell in the project directory.
2. Run the following command:
```bash
go test -v ./test/...
```

### Submitting Code
1. Fork project
2. Make changes
3. Submit Pull Request with passing tests