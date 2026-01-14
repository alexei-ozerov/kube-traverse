package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	infoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10")).Bold(true) // Green
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11")).Bold(true) // Yellow
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true)  // Red
	debugStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("13")).Bold(true) // Magenta
)

func colorizeLog(input string) string {
	input = strings.ReplaceAll(input, "INFO", infoStyle.Render("INFO"))
	input = strings.ReplaceAll(input, "WARN", warnStyle.Render("WARN"))
	input = strings.ReplaceAll(input, "ERROR", errorStyle.Render("ERROR"))
	input = strings.ReplaceAll(input, "DEBUG", debugStyle.Render("DEBUG"))
	return input
}
