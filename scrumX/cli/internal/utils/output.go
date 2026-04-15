package utils

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
)

const (
	reset  = "\033[0m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	bold   = "\033[1m"
)

func StatusColor(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "closed", "resolved":
		return green + status + reset
	case "in_progress", "in review", "review":
		return yellow + status + reset
	case "todo", "open", "backlog":
		return blue + status + reset
	default:
		return status
	}
}

func ErrorText(msg string) string   { return red + msg + reset }
func SuccessText(msg string) string { return green + msg + reset }
func HeaderText(msg string) string  { return bold + msg + reset }

func PrintTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	_ = w.Flush()
}
