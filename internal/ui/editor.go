package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"gopkg.in/yaml.v3"

	"github.com/ekorunov/devctl/internal/config"
)

// editorField tracks which field is focused in the profile editor.
type editorField int

const (
	fieldName editorField = iota
	fieldCompose
	fieldServices
)

// profileEditor is the state for the profile create/edit form.
type profileEditor struct {
	active bool
	edit   bool // true = editing existing, false = creating new

	nameInput textinput.Model
	field     editorField

	// Compose file selection
	composeFiles    []string // available files
	composeSelected []bool   // toggle state
	composeCursor   int

	// Service selection (populated after compose files are selected)
	serviceNames    []string
	serviceSelected []bool
	serviceCursor   int

	// Original profile name (when editing)
	originalName string

	// Project directory
	dir string
}

func newProfileEditor() profileEditor {
	ti := textinput.New()
	ti.Placeholder = "profile-name"
	ti.CharLimit = 64
	ti.Width = 40
	ti.PromptStyle = accentStyle
	ti.Prompt = ""

	return profileEditor{
		nameInput: ti,
	}
}

// openCreate initializes the editor for creating a new profile.
func (pe *profileEditor) openCreate(dir string, existingProfiles []config.Profile) {
	pe.active = true
	pe.edit = false
	pe.dir = dir
	pe.field = fieldName
	pe.nameInput.SetValue("")
	pe.nameInput.Focus()
	pe.originalName = ""
	pe.composeCursor = 0
	pe.serviceCursor = 0

	pe.composeFiles = discoverComposeFilesFromDir(dir)
	pe.composeSelected = make([]bool, len(pe.composeFiles))
	pe.refreshServices()
}

// openEdit initializes the editor for editing an existing profile.
func (pe *profileEditor) openEdit(dir string, profile config.Profile) {
	pe.active = true
	pe.edit = true
	pe.dir = dir
	pe.field = fieldName
	pe.nameInput.SetValue(profile.Name)
	pe.nameInput.Focus()
	pe.originalName = profile.Name
	pe.composeCursor = 0
	pe.serviceCursor = 0

	pe.composeFiles = discoverComposeFilesFromDir(dir)
	pe.composeSelected = make([]bool, len(pe.composeFiles))

	// Pre-select compose files from profile
	selectedSet := make(map[string]bool)
	for _, f := range profile.Compose {
		selectedSet[f] = true
	}
	for i, f := range pe.composeFiles {
		pe.composeSelected[i] = selectedSet[f]
	}

	// Discover services from selected compose files, pre-select profile's services
	pe.refreshServices()
	profileSvcs := make(map[string]bool)
	for _, s := range profile.Services {
		profileSvcs[s] = true
	}
	for i, name := range pe.serviceNames {
		pe.serviceSelected[i] = profileSvcs[name]
	}
}

func (pe *profileEditor) close() {
	pe.active = false
	pe.nameInput.Blur()
}

// selectedComposeFiles returns the list of selected compose files.
func (pe *profileEditor) selectedComposeFiles() []string {
	var files []string
	for i, f := range pe.composeFiles {
		if pe.composeSelected[i] {
			files = append(files, f)
		}
	}
	return files
}

// selectedServices returns the list of selected services.
func (pe *profileEditor) selectedServiceNames() []string {
	var svcs []string
	for i, s := range pe.serviceNames {
		if pe.serviceSelected[i] {
			svcs = append(svcs, s)
		}
	}
	return svcs
}

// toProfile builds a Profile from the current editor state.
func (pe *profileEditor) toProfile() (config.Profile, error) {
	name := strings.TrimSpace(pe.nameInput.Value())
	if name == "" {
		return config.Profile{}, fmt.Errorf("profile name is required")
	}

	files := pe.selectedComposeFiles()
	if len(files) == 0 {
		return config.Profile{}, fmt.Errorf("select at least one compose file")
	}

	svcs := pe.selectedServiceNames()

	return config.Profile{
		Name:     name,
		Compose:  files,
		Services: svcs,
	}, nil
}

// update handles input for the profile editor.
func (pe *profileEditor) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			pe.close()
			return nil

		case "tab":
			// Cycle fields forward
			switch pe.field {
			case fieldName:
				pe.nameInput.Blur()
				pe.field = fieldCompose
			case fieldCompose:
				pe.field = fieldServices
			case fieldServices:
				pe.field = fieldName
				pe.nameInput.Focus()
				return textinput.Blink
			}
			return nil

		case "shift+tab":
			switch pe.field {
			case fieldName:
				pe.nameInput.Blur()
				pe.field = fieldServices
			case fieldCompose:
				pe.field = fieldName
				pe.nameInput.Focus()
				return textinput.Blink
			case fieldServices:
				pe.field = fieldCompose
			}
			return nil

		case "up", "k":
			if pe.field == fieldCompose && pe.composeCursor > 0 {
				pe.composeCursor--
			}
			if pe.field == fieldServices && pe.serviceCursor > 0 {
				pe.serviceCursor--
			}
			return nil

		case "down", "j":
			if pe.field == fieldCompose && pe.composeCursor < len(pe.composeFiles)-1 {
				pe.composeCursor++
			}
			if pe.field == fieldServices && pe.serviceCursor < len(pe.serviceNames)-1 {
				pe.serviceCursor++
			}
			return nil

		case " ":
			if pe.field == fieldCompose && len(pe.composeFiles) > 0 {
				pe.composeSelected[pe.composeCursor] = !pe.composeSelected[pe.composeCursor]
				pe.refreshServices() // update available services
			}
			if pe.field == fieldServices && len(pe.serviceNames) > 0 {
				pe.serviceSelected[pe.serviceCursor] = !pe.serviceSelected[pe.serviceCursor]
			}
			return nil
		}
	}

	// Pass to text input when name field is focused
	if pe.field == fieldName {
		var cmd tea.Cmd
		pe.nameInput, cmd = pe.nameInput.Update(msg)
		return cmd
	}

	return nil
}

// view renders the profile editor dialog.
func (pe *profileEditor) view(width int) string {
	title := "Create Profile"
	if pe.edit {
		title = "Edit Profile"
	}

	var b strings.Builder

	// Name field
	nameLabel := faintStyle.Render("name")
	if pe.field == fieldName {
		nameLabel = accentStyle.Render("name")
	}
	b.WriteString(nameLabel + "      " + pe.nameInput.View())
	b.WriteString("\n\n")

	// Compose files
	composeLabel := faintStyle.Render("compose")
	if pe.field == fieldCompose {
		composeLabel = accentStyle.Render("compose")
	}
	b.WriteString(composeLabel + "\n")

	if len(pe.composeFiles) == 0 {
		b.WriteString(faintStyle.Render("  no compose files found\n"))
	} else {
		for i, f := range pe.composeFiles {
			cursor := "  "
			if pe.field == fieldCompose && i == pe.composeCursor {
				cursor = "> "
			}
			check := "[ ]"
			if pe.composeSelected[i] {
				check = statusOKStyle.Render("[x]")
			}
			b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, check, f))
		}
	}
	b.WriteString("\n")

	// Services (optional)
	svcLabel := faintStyle.Render("services")
	if pe.field == fieldServices {
		svcLabel = accentStyle.Render("services")
	}
	b.WriteString(svcLabel + "  " + faintStyle.Render("(optional, limit which to start)") + "\n")

	if len(pe.serviceNames) == 0 {
		b.WriteString(faintStyle.Render("  select compose files first\n"))
	} else {
		for i, s := range pe.serviceNames {
			cursor := "  "
			if pe.field == fieldServices && i == pe.serviceCursor {
				cursor = "> "
			}
			check := "[ ]"
			if pe.serviceSelected[i] {
				check = statusOKStyle.Render("[x]")
			}
			b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, check, s))
		}
	}

	// Help
	help := "\n" + helpKeyStyle.Render("tab") + " " + helpDescStyle.Render("next field") +
		" " + helpSepStyle.Render("│") + " " +
		helpKeyStyle.Render("space") + " " + helpDescStyle.Render("toggle") +
		" " + helpSepStyle.Render("│") + " " +
		helpKeyStyle.Render("enter") + " " + helpDescStyle.Render("save") +
		" " + helpSepStyle.Render("│") + " " +
		helpKeyStyle.Render("esc") + " " + helpDescStyle.Render("cancel")
	b.WriteString(help)

	dialogWidth := 60
	if width > 70 {
		dialogWidth = width - 20
	}
	if dialogWidth > 80 {
		dialogWidth = 80
	}

	return dialogStyle.Width(dialogWidth).Render(
		accentStyle.Render(title) + "\n\n" + b.String(),
	)
}

// refreshServices parses selected compose files and updates the service list.
// Preserves existing selection state for services that still exist.
func (pe *profileEditor) refreshServices() {
	files := pe.selectedComposeFiles()

	// Collect all service names from selected compose files
	seen := make(map[string]bool)
	var names []string
	for _, f := range files {
		svcs := parseComposeServices(filepath.Join(pe.dir, f))
		for _, s := range svcs {
			if !seen[s] {
				seen[s] = true
				names = append(names, s)
			}
		}
	}
	sort.Strings(names)

	// Preserve old selection
	oldSelected := make(map[string]bool)
	for i, name := range pe.serviceNames {
		if pe.serviceSelected[i] {
			oldSelected[name] = true
		}
	}

	pe.serviceNames = names
	pe.serviceSelected = make([]bool, len(names))
	for i, name := range names {
		pe.serviceSelected[i] = oldSelected[name]
	}
	if pe.serviceCursor >= len(names) {
		pe.serviceCursor = 0
	}
}

// parseComposeServices reads a compose file and returns its service names.
func parseComposeServices(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	// Parse just the top-level "services" key
	var doc struct {
		Services map[string]yaml.Node `yaml:"services"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil
	}

	var names []string
	for name := range doc.Services {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// discoverComposeFilesFromDir scans a directory for compose files.
func discoverComposeFilesFromDir(dir string) []string {
	entries, err := os.ReadDir(dir)
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
