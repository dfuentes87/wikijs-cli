package cli

import "testing"

func TestReplacer(t *testing.T) {
	replace, err := newReplacer("old", "new", false, false)
	if err != nil {
		t.Fatal(err)
	}
	got, changed := replace("OLD value")
	if !changed || got != "new value" {
		t.Fatalf("got %q changed=%v", got, changed)
	}
}
