# devctl

Docker-first TUI for managing dev environments. Run multiple docker-compose profiles simultaneously — for example, databases in one profile and APIs in another.

Built with Go, [Bubble Tea](https://github.com/charmbracelet/bubbletea), and [Lip Gloss](https://github.com/charmbracelet/lipgloss).

![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go&logoColor=white)

## Features

- **Multi-profile** — run several profiles at once, each as an isolated compose stack
- **Auto-detection** — works without config by discovering compose files in the directory
- **Quick launcher** — command prompt (`:`) with autocomplete for fast operations
- **Live status** — see running/stopped state of every service, auto-refreshes every 30s
- **Log streaming** — tail logs for any service directly in the TUI
- **Profile editor** — create and edit profiles without leaving the terminal
- **Rebuild & pull** — rebuild images or pull updates with a single keystroke

## Install

```bash
go install github.com/ekorunov/devctl/cmd/devctl@latest
```

Or build from source:

```bash
make build
make install
```

## Usage

```bash
devctl              # run in current directory
devctl /path/to/dir # specify project directory
devctl init         # generate .devtool/docker.yaml config
```

## Configuration

Config lives in `.devtool/docker.yaml`:

```yaml
project: MyProject

profiles:
  - name: databases
    compose:
      - docker-compose.yml
    services:
      - postgres
      - redis

  - name: api
    compose:
      - docker-compose.api.yml
    services:
      - gateway
      - auth
```

Without config, devctl auto-detects compose files and builds profiles:
- `docker-compose.yml` + `docker-compose.override.yml` → `default` profile
- `docker-compose.*.yml` → separate profiles named by variant

## Key Bindings

| Key       | Action                     |
|-----------|----------------------------|
| `u`       | Start services (up)        |
| `d`       | Stop services (down)       |
| `r`       | Restart service / all      |
| `b`       | Rebuild with --force       |
| `l`       | Stream logs                |
| `Tab`     | Switch focus (profiles ↔ services) |
| `↑↓`/`jk` | Navigate                  |
| `Enter`   | Select profile             |
| `:`       | Open quick launcher        |
| `c`       | Create new profile         |
| `e`       | Edit current profile       |
| `q`       | Quit                       |

## Quick Launcher

Press `:` to open the command prompt with autocomplete:

```
:up api          # start specific service
:down            # stop current profile
:restart worker  # restart a service
:rebuild         # rebuild all with --force-recreate
:pull            # pull latest images
:logs redis      # stream logs for a service
:init            # generate config file
```

## License

MIT
