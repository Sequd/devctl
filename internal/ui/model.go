package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ekorunov/devctl/internal/config"
	"github.com/ekorunov/devctl/internal/docker"
)

const (
	minWidth      = 80
	minHeight     = 24
	autoRefreshInterval = 30 * time.Second
	statusHideDelay     = 3 * time.Second
)

// View mode
type viewMode int

const (
	viewMain viewMode = iota
	viewLogs
	viewLauncher
	viewHelp
	viewEditor
)

// Messages
type servicesMsg []docker.Service
type errMsg struct{ err error }
type statusMsg string
type logLineMsg string
type logDoneMsg struct{}
type clearStatusMsg struct{}
type autoRefreshMsg struct{}
type configReloadMsg struct{ cfg *config.Config }
type editorFinishedMsg struct{ err error }
type activeProfileMsg struct {
	idx    int
	active bool // true = started, false = stopped
	status string
}

func (e errMsg) Error() string { return e.err.Error() }

// Model is the main TUI model.
type Model struct {
	cfg      *config.Config
	dir      string
	width    int
	height   int
	quitting bool

	// Profiles
	profiles        []config.Profile
	profileCursor   int
	selectedProfile int
	activeProfiles map[int]bool // profiles that have running containers

	// Services
	services      []docker.Service
	serviceCursor int

	// Focus: 0 = profiles, 1 = services
	focus int

	// Status bar (auto-hide)
	status string
	errStr string

	// View mode
	mode viewMode

	// Logs
	logLines   []string
	logCancel  context.CancelFunc
	logCmd     *exec.Cmd
	logService string
	logChan    chan string

	// Launcher
	launcher    textinput.Model
	suggestions []string
	suggIdx     int

	// Config state
	hasConfig bool

	// Help
	helpContent string

	// Profile editor
	editor profileEditor
}

// New creates a new TUI model.
func New(cfg *config.Config, dir string, hasConfig bool) Model {
	ti := textinput.New()
	ti.Placeholder = "up | down | restart | logs | edit | config add/rm | init"
	ti.CharLimit = 256
	ti.Width = 50
	ti.PromptStyle = launcherPromptStyle
	ti.Prompt = "> "

	m := Model{
		cfg:             cfg,
		dir:             dir,
		profiles:        cfg.Profiles,
		selectedProfile: -1,
		activeProfiles:  make(map[int]bool),
		launcher:        ti,
		width:           minWidth,
		height:          minHeight,
		hasConfig:       hasConfig,
		editor:          newProfileEditor(),
	}

	if len(m.profiles) > 0 {
		m.selectedProfile = 0
	}

	return m
}

func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{scheduleAutoRefresh()}
	if m.selectedProfile >= 0 {
		cmds = append(cmds, m.refreshServices())
	}
	if !m.hasConfig {
		cmds = append(cmds, func() tea.Msg {
			return statusMsg("Auto-detected profiles. Run :init to save config.")
		})
	}
	return tea.Batch(cmds...)
}

// --- Update ---

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = max(msg.Width, minWidth)
		m.height = max(msg.Height, minHeight)
		return m, nil

	case servicesMsg:
		m.services = []docker.Service(msg)
		// Track which profile has running containers
		hasRunning := false
		for _, svc := range msg {
			if svc.IsRunning() {
				hasRunning = true
				break
			}
		}
		if hasRunning {
			m.activeProfiles[m.selectedProfile] = true
		} else {
			delete(m.activeProfiles, m.selectedProfile)
		}
		return m, nil

	case errMsg:
		m.errStr = msg.Error()
		m.status = ""
		return m, scheduleClearStatus()

	case statusMsg:
		m.status = string(msg)
		m.errStr = ""
		if strings.HasPrefix(string(msg), "Config saved:") {
			m.hasConfig = true
		}
		return m, tea.Batch(m.refreshServices(), scheduleClearStatus())

	case activeProfileMsg:
		if msg.active {
			m.activeProfiles[msg.idx] = true
		} else {
			delete(m.activeProfiles, msg.idx)
		}
		m.status = msg.status
		m.errStr = ""
		return m, tea.Batch(m.refreshServices(), scheduleClearStatus())

	case clearStatusMsg:
		m.status = ""
		m.errStr = ""
		return m, nil

	case autoRefreshMsg:
		return m, tea.Batch(m.refreshServices(), scheduleAutoRefresh())

	case editorFinishedMsg:
		if msg.err != nil {
			m.errStr = msg.err.Error()
			return m, scheduleClearStatus()
		}
		// Reload config after editor closes
		return m, m.reloadConfig()

	case configReloadMsg:
		m.cfg = msg.cfg
		m.profiles = msg.cfg.Profiles
		m.hasConfig = true
		if m.selectedProfile >= len(m.profiles) {
			m.selectedProfile = 0
		}
		if m.profileCursor >= len(m.profiles) {
			m.profileCursor = 0
		}
		// Close editor if open
		if m.editor.active {
			m.editor.close()
			m.mode = viewMain
		}
		m.status = "Config saved"
		return m, tea.Batch(m.refreshServices(), scheduleClearStatus())

	case logLineMsg:
		m.logLines = append(m.logLines, string(msg))
		if len(m.logLines) > 500 {
			m.logLines = m.logLines[len(m.logLines)-500:]
		}
		return m, m.waitForLogLine()

	case logDoneMsg:
		return m, nil
	}

	switch m.mode {
	case viewLauncher:
		return m.updateLauncher(msg)
	case viewLogs:
		return m.updateLogs(msg)
	case viewHelp:
		return m.updateHelp(msg)
	case viewEditor:
		return m.updateEditor(msg)
	default:
		return m.updateMain(msg)
	}
}

func (m Model) updateMain(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			m.quitting = true
			return m, tea.Quit

		case key.Matches(msg, keys.Tab):
			if len(m.services) > 0 {
				m.focus = 1 - m.focus
			}
			return m, nil

		case key.Matches(msg, keys.Up):
			if m.focus == 0 {
				if m.profileCursor > 0 {
					m.profileCursor--
				}
			} else {
				if m.serviceCursor > 0 {
					m.serviceCursor--
				}
			}
			return m, nil

		case key.Matches(msg, keys.Down):
			if m.focus == 0 {
				if m.profileCursor < len(m.profiles)-1 {
					m.profileCursor++
				}
			} else {
				if m.serviceCursor < len(m.services)-1 {
					m.serviceCursor++
				}
			}
			return m, nil

		case key.Matches(msg, keys.Enter):
			if m.focus == 0 && len(m.profiles) > 0 {
				m.selectedProfile = m.profileCursor
				m.serviceCursor = 0
				m.services = nil
				return m, m.refreshServices()
			}
			return m, nil

		case key.Matches(msg, keys.DocUp):
			return m, m.execUp(nil)

		case key.Matches(msg, keys.DocDown):
			return m, m.execDown()

		case key.Matches(msg, keys.Restart):
			if m.focus == 1 && m.serviceCursor < len(m.services) {
				svc := m.services[m.serviceCursor].Name
				return m, m.execRestart([]string{svc})
			}
			return m, m.execRestart(nil)

		case key.Matches(msg, keys.Logs):
			if m.focus == 1 && m.serviceCursor < len(m.services) {
				svc := m.services[m.serviceCursor].Name
				return m, m.startLogs(svc)
			}
			return m, nil

		case key.Matches(msg, keys.Rebuild):
			if m.focus == 1 && m.serviceCursor < len(m.services) {
				svc := m.services[m.serviceCursor].Name
				return m, m.execRebuild([]string{svc})
			}
			return m, m.execRebuild(nil)

		case key.Matches(msg, keys.Command):
			m.mode = viewLauncher
			m.launcher.Focus()
			m.launcher.SetValue("")
			m.suggestions = m.completions("")
			m.suggIdx = 0
			return m, textinput.Blink

		case key.Matches(msg, keys.Create):
			m.editor.openCreate(m.dir, m.profiles)
			m.mode = viewEditor
			return m, textinput.Blink

		case key.Matches(msg, keys.Edit):
			if m.focus == 0 && m.selectedProfile >= 0 && m.selectedProfile < len(m.profiles) {
				m.editor.openEdit(m.dir, m.profiles[m.selectedProfile])
				m.mode = viewEditor
				return m, textinput.Blink
			}
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.mode = viewMain
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.editor.close()
			m.mode = viewMain
			return m, nil
		case "enter":
			if m.editor.field != fieldName {
				// Save the profile
				return m, m.saveEditorProfile()
			}
		}
	}

	cmd := m.editor.update(msg)
	return m, cmd
}

func (m Model) saveEditorProfile() tea.Cmd {
	profile, err := m.editor.toProfile()
	if err != nil {
		return func() tea.Msg { return errMsg{err} }
	}

	// Check name conflict (skip if editing same profile)
	for _, p := range m.profiles {
		if p.Name == profile.Name && profile.Name != m.editor.originalName {
			return func() tea.Msg {
				return errMsg{fmt.Errorf("profile %q already exists", profile.Name)}
			}
		}
	}

	dir := m.dir
	cfg := m.cfg
	isEdit := m.editor.edit
	origName := m.editor.originalName

	return func() tea.Msg {
		var newProfiles []config.Profile
		if isEdit {
			for _, p := range cfg.Profiles {
				if p.Name == origName {
					newProfiles = append(newProfiles, profile)
				} else {
					newProfiles = append(newProfiles, p)
				}
			}
		} else {
			newProfiles = append(cfg.Profiles, profile)
		}

		newCfg := &config.Config{
			Project:  cfg.Project,
			Profiles: newProfiles,
		}

		if err := config.Save(dir, newCfg); err != nil {
			return errMsg{err}
		}

		return configReloadMsg{cfg: newCfg}
	}
}

func (m Model) updateLogs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "esc", "ctrl+c":
			m.stopLogs()
			m.mode = viewMain
			return m, nil
		}
	}
	return m, nil
}

func (m Model) updateLauncher(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.mode = viewMain
			m.launcher.Blur()
			m.suggestions = nil
			return m, nil
		case "enter":
			cmd := m.launcher.Value()
			m.mode = viewMain
			m.launcher.Blur()
			m.suggestions = nil
			return m, m.execLauncherCmd(cmd)
		case "tab":
			if len(m.suggestions) > 0 {
				val := m.suggestions[m.suggIdx]
				// If this command expects more args, add trailing space
				if m.hasNextLevel(val) {
					val += " "
				}
				m.launcher.SetValue(val)
				m.launcher.CursorEnd()
				m.suggIdx = (m.suggIdx + 1) % len(m.suggestions)
				// Recalculate suggestions for the new value
				m.suggestions = m.completions(m.launcher.Value())
				m.suggIdx = 0
			}
			return m, nil
		case "shift+tab":
			if len(m.suggestions) > 0 {
				m.suggIdx--
				if m.suggIdx < 0 {
					m.suggIdx = len(m.suggestions) - 1
				}
				val := m.suggestions[m.suggIdx]
				if m.hasNextLevel(val) {
					val += " "
				}
				m.launcher.SetValue(val)
				m.launcher.CursorEnd()
				m.suggestions = m.completions(m.launcher.Value())
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.launcher, cmd = m.launcher.Update(msg)

	// Update suggestions after every keystroke
	m.suggestions = m.completions(m.launcher.Value())
	m.suggIdx = 0

	return m, cmd
}

// --- Autocomplete ---

// hasNextLevel returns true if the given value expects further arguments.
func (m Model) hasNextLevel(val string) bool {
	parts := strings.Fields(val)
	if len(parts) == 0 {
		return false
	}
	// Single commands that take service args
	if len(parts) == 1 {
		switch parts[0] {
		case "up", "restart", "rebuild", "pull", "logs", "config", "help":
			return true
		}
	}
	// "config add" / "config rm" take more args
	if len(parts) == 2 && parts[0] == "config" {
		return true
	}
	return false
}

// completions returns suggestions for the current launcher input.
func (m Model) completions(input string) []string {
	input = strings.TrimSpace(input)
	parts := strings.Fields(input)

	commands := []string{"up", "down", "restart", "rebuild", "pull", "logs"}
	if !m.hasConfig {
		commands = append(commands, "init")
	}
	if m.hasConfig {
		commands = append(commands, "edit")
	}
	commands = append(commands, "config", "help")

	// No input — suggest all commands
	if len(parts) == 0 {
		return commands
	}

	// Typing first word — filter commands by prefix
	if len(parts) == 1 && !strings.HasSuffix(input, " ") {
		prefix := parts[0]
		var matches []string
		for _, cmd := range commands {
			if strings.HasPrefix(cmd, prefix) && cmd != prefix {
				matches = append(matches, cmd)
			}
		}
		return matches
	}

	// Typing second word — context-dependent suggestions
	cmd := parts[0]
	switch cmd {
	case "up", "restart", "rebuild", "pull", "logs":
		var prefix string
		if len(parts) > 1 && !strings.HasSuffix(input, " ") {
			prefix = parts[len(parts)-1]
		}

		svcNames := m.serviceNames()
		var matches []string
		for _, name := range svcNames {
			if prefix == "" || strings.HasPrefix(name, prefix) {
				full := cmd + " " + name
				if full != input {
					matches = append(matches, full)
				}
			}
		}
		return matches

	case "config":
		subcommands := []string{"add", "rm"}
		if len(parts) == 1 && strings.HasSuffix(input, " ") {
			var matches []string
			for _, sc := range subcommands {
				matches = append(matches, "config "+sc)
			}
			return matches
		}
		if len(parts) == 2 && !strings.HasSuffix(input, " ") {
			prefix := parts[1]
			var matches []string
			for _, sc := range subcommands {
				if strings.HasPrefix(sc, prefix) && sc != prefix {
					matches = append(matches, "config "+sc)
				}
			}
			return matches
		}
		// After "config rm " — suggest existing profile names
		if len(parts) >= 2 && parts[1] == "rm" {
			var prefix string
			if len(parts) > 2 && !strings.HasSuffix(input, " ") {
				prefix = parts[len(parts)-1]
			}
			var matches []string
			for _, p := range m.profiles {
				if prefix == "" || strings.HasPrefix(p.Name, prefix) {
					matches = append(matches, "config rm "+p.Name)
				}
			}
			return matches
		}
		// After "config add <name> " — suggest compose files
		if len(parts) >= 3 && parts[1] == "add" {
			base := strings.Join(parts[:3], " ")
			// Already have files listed — suggest more
			if strings.HasSuffix(input, " ") {
				base = input
			}
			files := m.discoverComposeFiles()
			// Exclude files already in the command
			used := make(map[string]bool)
			for _, p := range parts[3:] {
				used[p] = true
			}
			var matches []string
			for _, f := range files {
				if !used[f] {
					matches = append(matches, strings.TrimSpace(base)+" "+f)
				}
			}
			return matches
		}
	case "help":
		topics := []string{"config", "up", "down", "restart", "rebuild", "pull", "logs", "edit", "init"}
		var prefix string
		if len(parts) > 1 && !strings.HasSuffix(input, " ") {
			prefix = parts[1]
		}
		var matches []string
		for _, t := range topics {
			if prefix == "" || strings.HasPrefix(t, prefix) {
				full := "help " + t
				if full != input {
					matches = append(matches, full)
				}
			}
		}
		return matches
	}

	return nil
}

// discoverComposeFiles returns compose file names in the project directory.
func (m Model) discoverComposeFiles() []string {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		if (strings.HasPrefix(name, "docker-compose") || strings.HasPrefix(name, "compose")) &&
			(strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) {
			files = append(files, e.Name())
		}
	}
	return files
}

// serviceNames returns the names of currently known services.
func (m Model) serviceNames() []string {
	names := make([]string, len(m.services))
	for i, svc := range m.services {
		names[i] = svc.Name
	}
	return names
}

// --- Commands ---

func scheduleAutoRefresh() tea.Cmd {
	return tea.Tick(autoRefreshInterval, func(time.Time) tea.Msg {
		return autoRefreshMsg{}
	})
}

func scheduleClearStatus() tea.Cmd {
	return tea.Tick(statusHideDelay, func(time.Time) tea.Msg {
		return clearStatusMsg{}
	})
}

func (m Model) currentProfile() *config.Profile {
	if m.selectedProfile < 0 || m.selectedProfile >= len(m.profiles) {
		return nil
	}
	return &m.profiles[m.selectedProfile]
}

// composeOptsFor builds ComposeOpts for a given profile index.
func (m Model) composeOptsFor(idx int) *docker.ComposeOpts {
	if idx < 0 || idx >= len(m.profiles) {
		return nil
	}
	p := &m.profiles[idx]
	project := m.cfg.Project + "-" + p.Name
	return &docker.ComposeOpts{
		Dir:     m.dir,
		Files:   p.Compose,
		Project: project,
	}
}

// composeOpts builds ComposeOpts for the current profile.
func (m Model) composeOpts() *docker.ComposeOpts {
	return m.composeOptsFor(m.selectedProfile)
}

func (m Model) refreshServices() tea.Cmd {
	opts := m.composeOpts()
	if opts == nil {
		return nil
	}
	return func() tea.Msg {
		svcs, err := docker.PS(context.Background(), *opts)
		if err != nil {
			return errMsg{err}
		}
		return servicesMsg(svcs)
	}
}

func (m Model) execUp(services []string) tea.Cmd {
	opts := m.composeOpts()
	if opts == nil {
		return nil
	}
	p := m.currentProfile()
	svcs := services
	if svcs == nil {
		svcs = p.Services
	}

	selected := m.selectedProfile

	return func() tea.Msg {
		if err := docker.Up(context.Background(), *opts, svcs); err != nil {
			return errMsg{err}
		}
		return activeProfileMsg{idx: selected, active: true, status: "Services started"}
	}
}

func (m Model) execDown() tea.Cmd {
	opts := m.composeOpts()
	if opts == nil {
		return nil
	}
	selected := m.selectedProfile
	return func() tea.Msg {
		if err := docker.Down(context.Background(), *opts); err != nil {
			return errMsg{err}
		}
		return activeProfileMsg{idx: selected, active: false, status: "Services stopped"}
	}
}

func (m Model) execRestart(services []string) tea.Cmd {
	opts := m.composeOpts()
	if opts == nil {
		return nil
	}
	return func() tea.Msg {
		if err := docker.Restart(context.Background(), *opts, services); err != nil {
			return errMsg{err}
		}
		name := "all"
		if len(services) > 0 {
			name = strings.Join(services, ", ")
		}
		return statusMsg(fmt.Sprintf("Restarted: %s", name))
	}
}

func (m Model) execRebuild(services []string) tea.Cmd {
	opts := m.composeOpts()
	if opts == nil {
		return nil
	}
	selected := m.selectedProfile
	return func() tea.Msg {
		if err := docker.Rebuild(context.Background(), *opts, services); err != nil {
			return errMsg{err}
		}
		name := "all"
		if len(services) > 0 {
			name = strings.Join(services, ", ")
		}
		return activeProfileMsg{idx: selected, active: true, status: fmt.Sprintf("Rebuilt: %s", name)}
	}
}

func (m Model) execPull(services []string) tea.Cmd {
	opts := m.composeOpts()
	if opts == nil {
		return nil
	}
	return func() tea.Msg {
		if err := docker.Pull(context.Background(), *opts, services); err != nil {
			return errMsg{err}
		}
		return statusMsg("Images pulled")
	}
}

func (m Model) execInit() tea.Cmd {
	if m.hasConfig {
		return func() tea.Msg { return errMsg{fmt.Errorf(".devtool/docker.yaml already exists")} }
	}
	dir := m.dir
	cfg := m.cfg
	return func() tea.Msg {
		path, err := config.Init(dir, cfg)
		if err != nil {
			return errMsg{err}
		}
		return statusMsg(fmt.Sprintf("Config saved: %s", path))
	}
}

// --- Help ---

var helpTopics = map[string]string{
	"": `Commands:
  up [service]                  Start services (all or specific)
  down                          Stop and remove all services
  restart [service]             Restart services (all or specific)
  rebuild [service]             Rebuild images and recreate containers
  pull [service]                Pull latest images
  logs <service>                Stream live logs for a service
  init                          Create .devtool/docker.yaml from detected files
  edit                          Open config in $EDITOR
  config add <name> <files...>  Add a new profile
  config rm <name>              Remove a profile
  help [topic]                  Show this help

Keys: u up | d down | r restart | b rebuild | l logs | c create | e edit

Config file: .devtool/docker.yaml
Type :help config for config format details.`,

	"config": `Config file: .devtool/docker.yaml

Format:
  project: MyProject          # project name (shown in header)
  profiles:
    - name: api-local         # profile name
      compose:                # list of compose files
        - docker-compose.yml
        - docker-compose.override.yml
      services:               # (optional) only start these services
        - api
        - redis

    - name: full-stack
      compose:
        - docker-compose.full.yml

Commands:
  :config add <name> <file1.yml> [file2.yml ...]
      Add a new profile with given compose files.
  :config rm <name>
      Remove an existing profile.
  :edit
      Open the config in $EDITOR. Changes are reloaded automatically.
  :init
      Auto-detect compose files and create initial config.

Fields:
  project     Display name for the project
  profiles    List of named environments
    name      Profile name (shown in sidebar)
    compose   Compose files to use (passed as -f to docker compose)
    services  Optional: limit which services to start with 'up'`,

	"up": `up [service...]
  Start services defined in the active profile.
  Without arguments: starts all services (or those in 'services' list).
  With arguments: starts only the named services.
  Example: :up api redis`,

	"down": `down
  Stop and remove all containers for the active profile.
  Runs: docker compose -f <files> down`,

	"restart": `restart [service...]
  Restart services in the active profile.
  Without arguments: restarts all services.
  With arguments: restarts only the named services.
  Example: :restart api`,

	"logs": `logs <service>
  Stream live logs for a specific service.
  Shows the last 100 lines, then follows.
  Press q or esc to return.
  Example: :logs redis`,

	"rebuild": `rebuild [service...]
  Rebuild images and recreate containers (--build --force-recreate).
  Use after changing Dockerfile or build context.
  Without arguments: rebuilds all services.
  Key: b (on service panel)
  Example: :rebuild api`,

	"pull": `pull [service...]
  Pull latest images for services.
  Use when remote images have been updated.
  Without arguments: pulls all.
  Example: :pull redis postgres`,

	"edit": `edit
  Open .devtool/docker.yaml in your editor.
  Uses $EDITOR, $VISUAL, or falls back to vim/nano/notepad.
  After closing the editor, the config is reloaded automatically.`,

	"init": `init
  Auto-detect compose files in the project directory and
  generate .devtool/docker.yaml with discovered profiles.
  Will not overwrite an existing config.`,
}

func (m *Model) execHelp(args []string) tea.Cmd {
	topic := ""
	if len(args) > 0 {
		topic = args[0]
	}

	text, ok := helpTopics[topic]
	if !ok {
		return func() tea.Msg {
			return errMsg{fmt.Errorf("unknown help topic: %s. Try :help", topic)}
		}
	}

	m.helpContent = text
	m.mode = viewHelp
	return nil
}

func (m Model) reloadConfig() tea.Cmd {
	dir := m.dir
	return func() tea.Msg {
		cfg, err := config.Load(dir)
		if err != nil {
			return errMsg{err}
		}
		if cfg == nil {
			return errMsg{fmt.Errorf("config file not found after edit")}
		}
		return configReloadMsg{cfg: cfg}
	}
}

func (m Model) execEdit() tea.Cmd {
	if !m.hasConfig {
		return func() tea.Msg {
			return errMsg{fmt.Errorf("no config to edit. Run :init first")}
		}
	}

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		// Fallback chain
		for _, name := range []string{"vim", "vi", "nano", "notepad"} {
			if _, err := exec.LookPath(name); err == nil {
				editor = name
				break
			}
		}
	}
	if editor == "" {
		return func() tea.Msg {
			return errMsg{fmt.Errorf("no $EDITOR set and no editor found")}
		}
	}

	path := config.Path(m.dir)
	c := exec.Command(editor, path)
	return tea.ExecProcess(c, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func (m Model) execConfigAdd(args []string) tea.Cmd {
	if len(args) == 0 {
		return func() tea.Msg {
			return errMsg{fmt.Errorf("usage: config add <profile-name> <file1.yml> [file2.yml ...]")}
		}
	}

	name := args[0]
	var files []string
	if len(args) > 1 {
		files = args[1:]
	}

	// Check for duplicate
	for _, p := range m.profiles {
		if p.Name == name {
			return func() tea.Msg {
				return errMsg{fmt.Errorf("profile %q already exists", name)}
			}
		}
	}

	dir := m.dir
	cfg := m.cfg

	return func() tea.Msg {
		// If no files specified, try to discover
		if len(files) == 0 {
			return errMsg{fmt.Errorf("usage: config add <profile-name> <file1.yml> [file2.yml ...]")}
		}

		// Verify files exist
		for _, f := range files {
			path := f
			if !filepath.IsAbs(path) {
				path = filepath.Join(dir, f)
			}
			if _, err := os.Stat(path); err != nil {
				return errMsg{fmt.Errorf("file not found: %s", f)}
			}
		}

		newCfg := &config.Config{
			Project:  cfg.Project,
			Profiles: append(cfg.Profiles, config.Profile{Name: name, Compose: files}),
		}

		if err := config.Save(dir, newCfg); err != nil {
			return errMsg{err}
		}

		return configReloadMsg{cfg: newCfg}
	}
}

func (m Model) execConfigRm(args []string) tea.Cmd {
	if len(args) == 0 {
		return func() tea.Msg {
			return errMsg{fmt.Errorf("usage: config rm <profile-name>")}
		}
	}

	name := args[0]
	found := false
	var remaining []config.Profile
	for _, p := range m.cfg.Profiles {
		if p.Name == name {
			found = true
			continue
		}
		remaining = append(remaining, p)
	}

	if !found {
		return func() tea.Msg {
			return errMsg{fmt.Errorf("profile %q not found", name)}
		}
	}

	if len(remaining) == 0 {
		return func() tea.Msg {
			return errMsg{fmt.Errorf("cannot remove last profile")}
		}
	}

	dir := m.dir
	cfg := m.cfg

	return func() tea.Msg {
		newCfg := &config.Config{
			Project:  cfg.Project,
			Profiles: remaining,
		}
		if err := config.Save(dir, newCfg); err != nil {
			return errMsg{err}
		}
		return configReloadMsg{cfg: newCfg}
	}
}

func (m *Model) startLogs(service string) tea.Cmd {
	opts := m.composeOpts()
	if opts == nil {
		return nil
	}

	m.stopLogs()
	m.mode = viewLogs
	m.logLines = nil
	m.logService = service

	ctx, cancel := context.WithCancel(context.Background())
	m.logCancel = cancel

	ch := make(chan string, 100)
	m.logChan = ch

	composeOpts := *opts

	go func() {
		reader, cmd, err := docker.Logs(ctx, composeOpts, service)
		if err != nil {
			ch <- "Error: " + err.Error()
			close(ch)
			return
		}
		_ = cmd
		buf := make([]byte, 4096)
		for {
			n, readErr := reader.Read(buf)
			if n > 0 {
				lines := strings.Split(string(buf[:n]), "\n")
				for _, line := range lines {
					if line != "" {
						select {
						case ch <- line:
						case <-ctx.Done():
							close(ch)
							return
						}
					}
				}
			}
			if readErr != nil {
				close(ch)
				return
			}
		}
	}()

	return waitForLog(ch)
}

func (m *Model) stopLogs() {
	if m.logCancel != nil {
		m.logCancel()
		m.logCancel = nil
	}
	if m.logCmd != nil {
		_ = m.logCmd.Process.Kill()
		m.logCmd = nil
	}
}

func waitForLog(ch chan string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return logDoneMsg{}
		}
		return logLineMsg(line)
	}
}

func (m Model) waitForLogLine() tea.Cmd {
	if m.logChan == nil {
		return nil
	}
	return waitForLog(m.logChan)
}

func (m Model) execLauncherCmd(input string) tea.Cmd {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return nil
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "up":
		return m.execUp(args)
	case "down":
		return m.execDown()
	case "restart":
		return m.execRestart(args)
	case "rebuild":
		return m.execRebuild(args)
	case "pull":
		return m.execPull(args)
	case "logs":
		if len(args) > 0 {
			return m.startLogs(args[0])
		}
		return func() tea.Msg { return errMsg{fmt.Errorf("logs requires a service name")} }
	case "init":
		return m.execInit()
	case "edit":
		return m.execEdit()
	case "config":
		if len(args) == 0 {
			return func() tea.Msg { return errMsg{fmt.Errorf("usage: config <add|rm> ...")} }
		}
		switch args[0] {
		case "add":
			return m.execConfigAdd(args[1:])
		case "rm":
			return m.execConfigRm(args[1:])
		default:
			return func() tea.Msg { return errMsg{fmt.Errorf("unknown config subcommand: %s", args[0])} }
		}
	case "help":
		return m.execHelp(args)
	default:
		return func() tea.Msg { return errMsg{fmt.Errorf("unknown command: %s", cmd)} }
	}
}

// --- View ---

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	switch m.mode {
	case viewLogs:
		return m.viewLogs()
	case viewLauncher:
		return m.viewMainWithLauncher()
	case viewHelp:
		return m.viewHelp()
	case viewEditor:
		return m.viewEditor()
	default:
		return m.viewMain()
	}
}

func (m Model) viewMain() string {
	return m.renderMain("")
}

func (m Model) viewMainWithLauncher() string {
	var footer strings.Builder
	footer.WriteString(m.launcher.View())

	if len(m.suggestions) > 0 {
		footer.WriteString("\n")
		for i, s := range m.suggestions {
			if i >= 5 { // show max 5 suggestions
				break
			}
			if i == m.suggIdx {
				footer.WriteString(accentStyle.Render("  "+s) + "\n")
			} else {
				footer.WriteString(faintStyle.Render("  "+s) + "\n")
			}
		}
		footer.WriteString(faintStyle.Render("  tab complete"))
	}

	return m.renderMain(footer.String())
}

func (m Model) renderMain(footer string) string {
	// Header
	header := titleStyle.Render("devctl") + "  " + subtitleStyle.Render(m.cfg.Project)

	// Two-column body
	leftWidth := m.width * 40 / 100
	rightWidth := m.width - leftWidth - 3 // 3 for " │ "
	if leftWidth < 20 {
		leftWidth = 20
	}
	if rightWidth < 30 {
		rightWidth = 30
	}

	bodyHeight := m.height - 4 // header + status + help + padding
	if bodyHeight < 10 {
		bodyHeight = 10
	}

	leftCol := m.renderProfiles(leftWidth, bodyHeight)
	rightCol := m.renderServices(rightWidth, bodyHeight)

	sep := separatorStyle.Render("│")
	body := lipgloss.JoinHorizontal(lipgloss.Top,
		leftCol,
		" "+sep+" ",
		rightCol,
	)

	// Status bar — always 1 line to prevent layout jumping
	statusBar := m.renderStatus()
	if statusBar == "" {
		statusBar = " " // reserve the line
	}

	// Help bar — context-aware
	helpBar := m.renderHelp()

	// Footer (launcher input)
	parts := []string{header, "", body, statusBar}
	if footer != "" {
		parts = append(parts, footer)
	}
	parts = append(parts, helpBar)

	return lipgloss.JoinVertical(lipgloss.Left, parts...)
}

func (m Model) renderProfiles(width, height int) string {
	var b strings.Builder

	title := accentStyle.Render("Profiles")
	b.WriteString(title)
	b.WriteString("\n\n")

	for i, p := range m.profiles {
		isCursor := i == m.profileCursor && m.focus == 0
		isSelected := i == m.selectedProfile
		isRunning := m.activeProfiles[i]

		// Running indicator
		indicator := " "
		if isRunning {
			indicator = statusOKStyle.Render("●")
		}

		name := p.Name
		if isSelected {
			name = accentStyle.Render(p.Name)
		}

		if isCursor {
			line := fmt.Sprintf("> %s %s", indicator, p.Name)
			for len(line) < width {
				line += " "
			}
			b.WriteString(selectedRowStyle.Render(line))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s", indicator, name))
		}

		if i < len(m.profiles)-1 {
			b.WriteString("\n")
		}
	}

	if len(m.profiles) == 0 {
		b.WriteString(faintStyle.Render("  No profiles found"))
	}

	// Pad to fill height
	lines := strings.Count(b.String(), "\n") + 1
	for lines < height {
		b.WriteString("\n")
		lines++
	}

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m Model) renderServices(width, height int) string {
	var b strings.Builder

	p := m.currentProfile()
	if p != nil {
		title := accentStyle.Render("Services")
		b.WriteString(title + "  " + faintStyle.Render(p.Name))
	} else {
		b.WriteString(accentStyle.Render("Services"))
	}
	b.WriteString("\n\n")

	if len(m.services) == 0 {
		b.WriteString(faintStyle.Render("  No services running"))
		if p != nil {
			b.WriteString("\n")
			b.WriteString(faintStyle.Render("  Press u to start"))
		}
	} else {
		// Find max name length for alignment
		maxName := 0
		for _, svc := range m.services {
			if len(svc.Name) > maxName {
				maxName = len(svc.Name)
			}
		}

		for i, svc := range m.services {
			isCursor := i == m.serviceCursor && m.focus == 1

			// Status indicator
			var indicator string
			if svc.IsRunning() {
				indicator = statusOKStyle.Render("●")
			} else {
				indicator = statusErrStyle.Render("●")
			}

			// Padded name
			name := svc.Name
			padded := name + strings.Repeat(" ", maxName-len(name))

			// Extra info
			var extra string
			if svc.Ports != "" {
				extra += "  " + faintStyle.Render(svc.Ports)
			}
			if svc.RunTime != "" {
				extra += "  " + faintStyle.Render(svc.RunTime)
			}

			if isCursor {
				line := fmt.Sprintf("> %s %s%s", indicator, padded, extra)
				b.WriteString(selectedRowStyle.Render(line))
			} else {
				b.WriteString(fmt.Sprintf("  %s %s%s", indicator, textStyle.Render(padded), extra))
			}

			if i < len(m.services)-1 {
				b.WriteString("\n")
			}
		}
	}

	// Pad to fill height
	lines := strings.Count(b.String(), "\n") + 1
	for lines < height {
		b.WriteString("\n")
		lines++
	}

	return lipgloss.NewStyle().Width(width).Render(b.String())
}

func (m Model) renderStatus() string {
	if m.errStr != "" {
		return errorMsgStyle.Render("Error: " + m.errStr)
	}
	if m.status != "" {
		return successMsgStyle.Render(m.status)
	}
	return ""
}

func (m Model) renderHelp() string {
	var items []struct{ key, desc string }

	if m.focus == 0 {
		// Profile panel
		items = []struct{ key, desc string }{
			{"↑↓", "navigate"},
			{"enter", "select"},
			{"c", "create"},
			{"e", "edit"},
			{"u", "up"},
			{"d", "down"},
			{"tab", "services"},
			{":", "command"},
			{"q", "quit"},
		}
	} else {
		// Service panel
		items = []struct{ key, desc string }{
			{"↑↓", "navigate"},
			{"r", "restart"},
			{"b", "rebuild"},
			{"l", "logs"},
			{"u", "up"},
			{"d", "down"},
			{"c", "create"},
			{"tab", "profiles"},
			{":", "command"},
			{"q", "quit"},
		}
	}

	var parts []string
	for _, item := range items {
		parts = append(parts,
			helpKeyStyle.Render(item.key)+" "+helpDescStyle.Render(item.desc),
		)
	}

	sep := " " + helpSepStyle.Render("│") + " "
	return strings.Join(parts, sep)
}

func (m Model) viewLogs() string {
	header := titleStyle.Render("Logs") + "  " + subtitleStyle.Render(m.logService)

	maxLines := m.height - 4
	if maxLines < 10 {
		maxLines = 10
	}

	lines := m.logLines
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}

	content := strings.Join(lines, "\n")
	if content == "" {
		content = faintStyle.Render("Waiting for logs...")
	}

	help := helpKeyStyle.Render("q") + " " + helpDescStyle.Render("back") +
		" " + helpSepStyle.Render("│") + " " +
		helpKeyStyle.Render("esc") + " " + helpDescStyle.Render("back")

	return lipgloss.JoinVertical(lipgloss.Left, header, "", content, "", help)
}

func (m Model) viewHelp() string {
	header := titleStyle.Render("Help")

	help := helpKeyStyle.Render("q") + " " + helpDescStyle.Render("back") +
		" " + helpSepStyle.Render("│") + " " +
		helpKeyStyle.Render("esc") + " " + helpDescStyle.Render("back")

	return lipgloss.JoinVertical(lipgloss.Left, header, "", m.helpContent, "", help)
}

func (m Model) viewEditor() string {
	dialog := m.editor.view(m.width)

	// Center the dialog vertically
	padding := (m.height - lipgloss.Height(dialog)) / 2
	if padding < 0 {
		padding = 0
	}
	var top string
	for i := 0; i < padding; i++ {
		top += "\n"
	}

	return top + dialog
}
