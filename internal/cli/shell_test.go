package cli

import (
	"reflect"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	got, err := splitCommandLine(`create "/docs/new page" "New Page" --tag docs`)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"create", "/docs/new page", "New Page", "--tag", "docs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
