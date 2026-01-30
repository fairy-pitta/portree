package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	colorPrimary   = lipgloss.Color("#7C3AED") // purple
	colorGreen     = lipgloss.Color("#10B981")
	colorRed       = lipgloss.Color("#EF4444")
	colorYellow    = lipgloss.Color("#F59E0B")
	colorGray      = lipgloss.Color("#6B7280")
	colorDimGray   = lipgloss.Color("#374151")
	colorWhite     = lipgloss.Color("#F9FAFB")

	// Title bar
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			Background(colorPrimary).
			Padding(0, 1)

	// Table header
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorWhite).
			BorderBottom(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorDimGray)

	// Table row
	rowStyle = lipgloss.NewStyle().
			Foreground(colorWhite)

	// Selected row
	selectedRowStyle = lipgloss.NewStyle().
				Foreground(colorWhite).
				Background(colorDimGray).
				Bold(true)

	// Status indicators
	statusRunning = lipgloss.NewStyle().
			Foreground(colorGreen).
			Render("● running")

	statusStopped = lipgloss.NewStyle().
			Foreground(colorRed).
			Render("○ stopped")

	// Footer / help bar
	helpStyle = lipgloss.NewStyle().
			Foreground(colorGray).
			MarginTop(1)

	// Proxy status
	proxyRunningStyle = lipgloss.NewStyle().
				Foreground(colorGreen).
				Bold(true)

	proxyStoppedStyle = lipgloss.NewStyle().
				Foreground(colorRed)

	// Border for the whole dashboard
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
			Padding(1, 2)
)
