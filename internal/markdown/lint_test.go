package markdown

import "testing"

func TestLintFindsErrorsAndWarnings(t *testing.T) {
	result := Lint("##Bad\nline \n[broken](url")
	if result.Valid {
		t.Fatal("expected invalid document")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected errors")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected warnings")
	}
}

func TestLintValidDocument(t *testing.T) {
	result := Lint("# Title\n\nBody")
	if !result.Valid {
		t.Fatalf("expected valid, got %+v", result)
	}
}
