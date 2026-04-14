package ui

import "github.com/charmbracelet/lipgloss"

// Color palette — dark theme with one accent and three semantic colors.
var (
	colorAccent   = lipgloss.Color("#5F9FFF")
	colorText     = lipgloss.Color("#FFFFFF")
	colorMuted    = lipgloss.Color("#A0AEC0")
	colorFaint    = lipgloss.Color("#718096")
	colorBorder   = lipgloss.Color("#4A5568")
	colorSelectBg = lipgloss.Color("#2C5282")

	colorOK   = lipgloss.Color("#68D391")
	colorWarn = lipgloss.Color("#F6C950")
	colorErr  = lipgloss.Color("#FC8181")
)

// Styles
var (
	// Title: accent bold
	titleStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	// Subtitle / project name
	subtitleStyle = lipgloss.NewStyle().
			Foreground(colorFaint)

	// Normal text
	textStyle = lipgloss.NewStyle().
			Foreground(colorText)

	// Faint / secondary text
	faintStyle = lipgloss.NewStyle().
			Foreground(colorFaint)

	// Selected row: bold white on selectBg
	selectedRowStyle = lipgloss.NewStyle().
				Foreground(colorText).
				Background(colorSelectBg).
				Bold(true)

	// Active profile marker
	accentStyle = lipgloss.NewStyle().
			Foreground(colorAccent).
			Bold(true)

	// Status indicators
	statusOKStyle = lipgloss.NewStyle().
			Foreground(colorOK)

	statusWarnStyle = lipgloss.NewStyle().
			Foreground(colorWarn)

	statusErrStyle = lipgloss.NewStyle().
			Foreground(colorErr)

	// Column separator
	separatorStyle = lipgloss.NewStyle().
			Foreground(colorBorder)

	// Help bar: key = muted bold, desc = faint, sep = border
	helpKeyStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Bold(true)

	helpDescStyle = lipgloss.NewStyle().
			Foreground(colorFaint)

	helpSepStyle = lipgloss.NewStyle().
			Foreground(colorBorder)

	// Dialog border (only dialogs get borders)
	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 3).
			Width(60)

	// Launcher prompt
	launcherPromptStyle = lipgloss.NewStyle().
				Foreground(colorAccent).
				Bold(true)

	// Status messages
	successMsgStyle = lipgloss.NewStyle().
			Foreground(colorOK)

	errorMsgStyle = lipgloss.NewStyle().
			Foreground(colorErr).
			Bold(true)
)
