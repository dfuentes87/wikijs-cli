package markdown

import "testing"

func TestLinksIgnoresFencedCode(t *testing.T) {
	links := Links("```markdown\n[Missing](/missing)\n```\n[Real](/real)")
	if len(links) != 1 {
		t.Fatalf("links = %+v", links)
	}
	if links[0].Target != "/real" || links[0].Text != "Real" {
		t.Fatalf("unexpected link: %+v", links[0])
	}
}

func TestLinksFindsInternalReferences(t *testing.T) {
	links := Links("[Guide](/docs/guide)\n![Hero](/uploads/hero.png)")
	if len(links) != 2 {
		t.Fatalf("links = %+v", links)
	}
	if links[0].Image || links[0].Target != "/docs/guide" {
		t.Fatalf("unexpected page link: %+v", links[0])
	}
	if !links[1].Image || links[1].Target != "/uploads/hero.png" {
		t.Fatalf("unexpected image link: %+v", links[1])
	}
}

func TestLinksHandlesParenthesesInTargets(t *testing.T) {
	links := Links("[Guide](/docs/guide_(draft))")
	if len(links) != 1 {
		t.Fatalf("links = %+v", links)
	}
	if links[0].Target != "/docs/guide_(draft)" {
		t.Fatalf("target = %q", links[0].Target)
	}
}
