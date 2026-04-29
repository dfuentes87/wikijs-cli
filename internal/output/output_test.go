package output

import (
	"bytes"
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := JSON(&buf, map[string]string{"name": "wiki"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"name": "wiki"`) {
		t.Fatalf("unexpected json: %s", buf.String())
	}
}

func TestTable(t *testing.T) {
	var buf bytes.Buffer
	if err := Table(&buf, []string{"ID", "Name"}, [][]string{{"1", "Home"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "Home") {
		t.Fatalf("unexpected table: %s", buf.String())
	}
}

func TestColor(t *testing.T) {
	if got := Color(false, Green, "ok"); got != "ok" {
		t.Fatalf("disabled color = %q", got)
	}
	if got := Color(true, Green, "ok"); got != Green+"ok"+Reset {
		t.Fatalf("enabled color = %q", got)
	}
}
