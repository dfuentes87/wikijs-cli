package cli

import (
	"fmt"

	"github.com/dfuentes87/wikijs-cli/internal/output"
)

type operationSummary struct {
	Created int      `json:"created,omitempty"`
	Updated int      `json:"updated,omitempty"`
	Skipped int      `json:"skipped,omitempty"`
	Matched int      `json:"matched,omitempty"`
	Changed int      `json:"changed,omitempty"`
	Deleted int      `json:"deleted,omitempty"`
	Files   int      `json:"files,omitempty"`
	Pages   int      `json:"pages,omitempty"`
	Words   int      `json:"words,omitempty"`
	Paths   []string `json:"paths,omitempty"`
}

func (a *app) printSummary(action string, summary operationSummary) error {
	if a.format == "json" {
		return output.JSON(a.out, successResult{Success: true, Action: action, Result: summary})
	}
	_, err := fmt.Fprintf(a.out, "%s complete: %d created, %d updated, %d skipped, %d matched, %d changed\n",
		action, summary.Created, summary.Updated, summary.Skipped, summary.Matched, summary.Changed)
	return err
}

func (a *app) progress(label string, current, total int) {
	if total <= 0 {
		return
	}
	fmt.Fprintf(a.errOut, "\r%s: %d/%d", label, current, total)
}

func (a *app) progressDone() {
	fmt.Fprintln(a.errOut)
}

func strconvItoa(value int) string {
	return fmt.Sprintf("%d", value)
}
