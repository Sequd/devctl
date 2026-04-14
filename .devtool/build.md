## Go CLI Project: Build & Install Setup

### Requirements

- Entrypoint at `cmd/<app-name>/main.go`
- Binary name configurable via `BINARY_NAME` in Makefile
- OS auto-detection: `.exe` on Windows, no extension on Linux/macOS
- `install` copies binary to `$GOBIN` (defaults to `~/go/bin`)

### Commands

```bash
make build     # build binary to project root
make install   # build + copy to ~/go/bin
make run       # build + run
make clean     # remove binaries
make tidy      # go mod tidy
```

### Verify

```bash
make build              # binary appears in project root
make install            # binary copied to ~/go/bin
which <app-name>        # should resolve if ~/go/bin is in PATH
```

### Expected Layout

```
project-root/
├── cmd/
│   └── <app-name>/
│       └── main.go
├── internal/
├── go.mod
├── go.sum
└── Makefile
```
