package main

import (
	"bytes"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"

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

func highlightYAML(yamlContent string) string {
	lexer := lexers.Get("yaml")
	if lexer == nil {
		lexer = lexers.Fallback
	}

	// You can change "monokai" to "dracula", "solarized-dark", etc.
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}

	formatter := formatters.Get("terminal256")
	if formatter == nil {
		formatter = formatters.Fallback
	}

	iterator, err := lexer.Tokenise(nil, yamlContent)
	if err != nil {
		return yamlContent
	}

	var buf bytes.Buffer
	err = formatter.Format(&buf, style, iterator)
	if err != nil {
		return yamlContent
	}

	return buf.String()
}
