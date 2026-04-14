package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// Service represents a running docker compose service.
type Service struct {
	Name      string // compose service name (e.g. "grafana")
	Container string // container name (e.g. "project-grafana-1")
	Status    string // "running", "exited", "paused", etc.
	Ports     string
	RunTime   string
}

// IsRunning returns true if the service status indicates it's running.
func (s Service) IsRunning() bool {
	return strings.Contains(strings.ToLower(s.Status), "running") ||
		strings.Contains(strings.ToLower(s.Status), "up")
}

// ComposeOpts holds options for docker compose commands.
type ComposeOpts struct {
	Dir     string
	Files   []string
	Project string // -p flag; empty = docker compose default
}

// composeArgs builds the base docker compose args.
func composeArgs(opts ComposeOpts) []string {
	args := []string{"compose"}
	if opts.Project != "" {
		args = append(args, "-p", opts.Project)
	}
	for _, f := range opts.Files {
		args = append(args, "-f", f)
	}
	return args
}

// Up starts services. If services is empty, starts all.
func Up(ctx context.Context, opts ComposeOpts, services []string) error {
	args := composeArgs(opts)
	args = append(args, "up", "-d")
	args = append(args, services...)
	return run(ctx, opts.Dir, args)
}

// Down stops and removes services.
func Down(ctx context.Context, opts ComposeOpts) error {
	args := composeArgs(opts)
	args = append(args, "down")
	return run(ctx, opts.Dir, args)
}

// Restart restarts services.
func Restart(ctx context.Context, opts ComposeOpts, services []string) error {
	args := composeArgs(opts)
	args = append(args, "restart")
	args = append(args, services...)
	return run(ctx, opts.Dir, args)
}

// Rebuild rebuilds images and recreates containers.
func Rebuild(ctx context.Context, opts ComposeOpts, services []string) error {
	args := composeArgs(opts)
	args = append(args, "up", "-d", "--build", "--force-recreate")
	args = append(args, services...)
	return run(ctx, opts.Dir, args)
}

// Pull pulls latest images for services.
func Pull(ctx context.Context, opts ComposeOpts, services []string) error {
	args := composeArgs(opts)
	args = append(args, "pull")
	args = append(args, services...)
	return run(ctx, opts.Dir, args)
}

// PS returns the list of services and their status.
func PS(ctx context.Context, opts ComposeOpts) ([]Service, error) {
	args := composeArgs(opts)
	args = append(args, "ps", "--format", "{{.Service}}\t{{.Name}}\t{{.Status}}\t{{.Ports}}")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = opts.Dir
	out, err := cmd.Output()
	if err != nil {
		return nil, nil
	}

	return parsePS(string(out)), nil
}

func parsePS(output string) []Service {
	var services []Service
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 2 {
			continue
		}
		svc := Service{
			Name:      parts[0],
			Container: parts[1],
		}
		if len(parts) > 2 {
			svc.Status = parts[2]
			if idx := strings.Index(strings.ToLower(svc.Status), "up "); idx >= 0 {
				svc.RunTime = svc.Status[idx+3:]
			}
		}
		if len(parts) > 3 {
			svc.Ports = parts[3]
		}
		services = append(services, svc)
	}
	return services
}

// Logs streams logs for a service.
func Logs(ctx context.Context, opts ComposeOpts, service string) (io.ReadCloser, *exec.Cmd, error) {
	args := composeArgs(opts)
	args = append(args, "logs", "-f", "--tail", "100")
	if service != "" {
		args = append(args, service)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = opts.Dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}

	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start logs: %w", err)
	}

	return stdout, cmd, nil
}

func run(ctx context.Context, dir string, args []string) error {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", extractError(string(out)))
	}
	return nil
}

// extractError pulls the meaningful error from docker compose output,
// stripping progress lines (Container ... Starting/Recreated/Running).
func extractError(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")

	var errors []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip progress lines
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "container ") || strings.HasPrefix(lower, "network ") || strings.HasPrefix(lower, "volume ") {
			suffixes := []string{"creating", "created", "starting", "started",
				"stopping", "stopped", "removing", "removed",
				"running", "recreate", "recreated", "waiting"}
			isProgress := false
			for _, s := range suffixes {
				if strings.HasSuffix(lower, s) {
					isProgress = true
					break
				}
			}
			if isProgress {
				continue
			}
		}
		errors = append(errors, line)
	}

	if len(errors) == 0 {
		return output // fallback to full output
	}
	return strings.Join(errors, "\n")
}
