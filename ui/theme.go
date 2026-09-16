package ui

import "charm.land/lipgloss/v2"

// The TUI uses one small palette so each screen feels like part of the same
// terminal workspace. Every state still has a text or shape cue in addition
// to its color, which keeps the interface usable in reduced color profiles.
var (
	primaryTextColor   = lipgloss.Color("#E8EDF5")
	secondaryTextColor = lipgloss.Color("#B7C1D1")
	accentColor        = lipgloss.Color("#E46AC4")
	activeColor        = lipgloss.Color("#F29AD6")
	focusSurfaceColor  = lipgloss.Color("#3B2C40")
	queueBorderColor   = lipgloss.Color("#5F6573")
	mutedColor         = lipgloss.Color("#8E98AA")
	successColor       = lipgloss.Color("#A8DFB0")
	errorColor         = lipgloss.Color("#FF7D93")

	redForeground     = lipgloss.NewStyle().Foreground(errorColor)
	greenForeground   = lipgloss.NewStyle().Foreground(successColor)
	grayForeground    = lipgloss.NewStyle().Foreground(mutedColor)
	dimGrayForeground = lipgloss.NewStyle().Foreground(mutedColor)
	boldStyle         = lipgloss.NewStyle().Foreground(primaryTextColor).Bold(true)
	boldRedStyle      = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	spinnerStyle      = lipgloss.NewStyle().Foreground(accentColor)

	appBrandStyle      = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	sectionLabelStyle  = lipgloss.NewStyle().Foreground(mutedColor).Bold(true)
	screenTitleStyle   = lipgloss.NewStyle().Foreground(primaryTextColor).Bold(true)
	headerValueStyle   = lipgloss.NewStyle().Foreground(primaryTextColor).Bold(true)
	headerFormatStyle  = lipgloss.NewStyle().Foreground(activeColor).Bold(true)
	headerDividerStyle = lipgloss.NewStyle().Foreground(queueBorderColor)
)

func renderScreenHeader(section, title string) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		appBrandStyle.Render("yamdl"),
		headerDividerStyle.Render("  //  "),
		sectionLabelStyle.Render(section),
	) + "\n" + screenTitleStyle.Render(title)
}
