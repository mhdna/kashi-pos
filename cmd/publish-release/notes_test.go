package main

import "testing"

func TestReleaseNotesFromCommits(t *testing.T) {
	got := releaseNotes([]string{
		"chore: release v1.1.94",
		"Merge small-updates: patch and compressed downloads for till updates",
		"feat(updates): download a patch or the compressed build, not the whole exe",
		"fix(shutdown): no \"about to be signed out\" notice; tidier goodbye",
		"docs: rebuild developer docs",
		"fix(invoices): rename 'Change Salesman' to 'Change Salesperson'",
		"perf(sync): lists apply only what changed",
		"feat(invoices): show how clients rated each sale",
		"refactor: tidy",
		"no prefix at all",
	})
	want := "• New: Show how clients rated each sale\n" +
		"• New: Download a patch or the compressed build, not the whole exe\n" +
		"• Faster: Lists apply only what changed\n" +
		"• Fixed: Rename 'Change Salesman' to 'Change Salesperson'\n" +
		"• Fixed: No \"about to be signed out\" notice; tidier goodbye"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestReleaseNotesAreCapped(t *testing.T) {
	var subjects []string
	for i := 0; i < maxNoteLines+3; i++ {
		subjects = append(subjects, "fix: thing "+string(rune('a'+i)))
	}
	lines := releaseNotes(subjects)
	if want := "…and 3 more"; lines[len(lines)-len(want):] != want {
		t.Fatalf("not capped: %s", lines)
	}
}

func TestPreviousReleaseIsTheNewestBelowOfTheFlavor(t *testing.T) {
	rels := []publishedRelease{
		{Version: "1.1.90", Flavor: "standard"}, {Version: "1.1.93", Flavor: "standard"},
		{Version: "1.1.94", Flavor: "standard"}, {Version: "1.1.95", Flavor: "compact"},
		{Version: "1.1.100", Flavor: "standard"},
	}
	if got := previousRelease(rels, "standard", "1.1.95"); got != "1.1.94" {
		t.Fatalf("got %q", got)
	}
	if got := previousRelease(rels, "standard", "1.1.101"); got != "1.1.100" {
		t.Fatalf("numbers compare as numbers: got %q", got)
	}
	if got := previousRelease(rels, "compact", "1.1.95"); got != "" {
		t.Fatalf("got %q", got)
	}
}
