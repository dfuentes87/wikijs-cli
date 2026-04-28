package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"
)

func JSON(w io.Writer, data any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func Table(w io.Writer, headers []string, rows [][]string) error {
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No results found.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}
	return tw.Flush()
}

func Bool(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func Date(value string) string {
	if value == "" {
		return ""
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.Local().Format("2006-01-02 15:04")
	}
	return value
}

func Bytes(bytes int64) string {
	if bytes == 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	idx := 0
	for size >= 1024 && idx < len(units)-1 {
		size /= 1024
		idx++
	}
	if idx == 0 {
		return fmt.Sprintf("%d %s", bytes, units[idx])
	}
	return fmt.Sprintf("%.2f %s", size, units[idx])
}

func Truncate(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	if max <= 3 {
		return value[:max]
	}
	return value[:max-3] + "..."
}
