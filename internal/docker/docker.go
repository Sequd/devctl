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
	Health    string // "healthy", "unhealthy", "starting", "" (no healthcheck)
	Ports     string
	RunTime   string
}

// IsRunning returns true if the service status indicates it's running.
func (s Service) IsRunning() bool {
	return strings.Contains(strings.ToLower(s.Status), "running") ||
		strings.Contains(strings.ToLower(s.Status), "up")
}

// IsHealthy returns true if service has a healthcheck and it's healthy.
func (s Service) IsHealthy() bool {
	return strings.ToLower(s.Health) == "healthy"
}

// HasHealthCheck returns true if the service has a health check configured.
func (s Service) HasHealthCheck() bool {
	return s.Health != ""
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
	args = append(args, "ps", "-a", "--format", "{{.Service}}\t{{.Name}}\t{{.Status}}\t{{.Health}}\t{{.Ports}}")

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = opts.Dir
	out, err := cmd.Output()
	if err != nil {
		// If compose project has no containers yet, docker returns exit code 1
		// with empty output — this is not an error, just no services.
		if len(strings.TrimSpace(string(out))) == 0 {
			return nil, nil
		}
		exitErr, ok := err.(*exec.ExitError)
		if ok && len(exitErr.Stderr) > 0 {
			return nil, fmt.Errorf("%s", extractError(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("docker compose ps: %w", err)
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
		parts := strings.SplitN(line, "\t", 5)
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
				svc.RunTime = shortDuration(svc.Status[idx+3:])
			}
		}
		if len(parts) > 3 {
			svc.Health = parts[3]
		}
		if len(parts) > 4 {
			svc.Ports = parts[4]
		}
		services = append(services, svc)
	}
	return services
}

// shortDuration converts docker's verbose duration to compact form.
// "About a minute" → "~1m", "2 hours" → "2h", "30 seconds" → "30s",
// "3 minutes" → "3m", "About an hour" → "~1h"
func shortDuration(d string) string {
	d = strings.TrimSpace(d)
	lower := strings.ToLower(d)

	// Keep human-readable short phrases as-is
	if strings.HasPrefix(lower, "less than") {
		return d
	}

	// Strip "About " prefix
	approx := false
	if strings.HasPrefix(lower, "about ") {
		approx = true
		lower = strings.TrimPrefix(lower, "about ")
	}

	prefix := ""
	if approx {
		prefix = "~"
	}

	// "a minute", "an hour"
	if lower == "a minute" || lower == "1 minute" {
		return prefix + "1m"
	}
	if lower == "an hour" || lower == "1 hour" {
		return prefix + "1h"
	}

	// "N seconds/minutes/hours/days"
	parts := strings.Fields(lower)
	if len(parts) == 2 {
		num := parts[0]
		unit := parts[1]
		switch {
		case strings.HasPrefix(unit, "second"):
			return prefix + num + "s"
		case strings.HasPrefix(unit, "minute"):
			return prefix + num + "m"
		case strings.HasPrefix(unit, "hour"):
			return prefix + num + "h"
		case strings.HasPrefix(unit, "day"):
			return prefix + num + "d"
		}
	}

	// Fallback: return as-is but trimmed
	if approx {
		return "~" + d[6:]
	}
	return d
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
