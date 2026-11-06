package main

import (
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// When `make publish` gets no NOTES, the notes the cashiers see after the
// update are written from the commits since the build kashi last published
// for that flavor: "feat" lines as "New: …", "fix" as "Fixed: …", "perf" as
// "Faster: …". Merges, releases, docs, tests and refactors are left out.

// Optional "type(scope)!: " prefix, as kashi's cmd/release_notes reads it.
var commitSubjectRe = regexp.MustCompile(`^([a-z]+)(?:\([^)]*\))?!?:\s*(.+)$`)

var noteKinds = []struct{ kind, label string }{
	{"feat", "New"},
	{"perf", "Faster"},
	{"fix", "Fixed"},
}

// maxNoteLines keeps the dialog short; the rest is counted.
const maxNoteLines = 12

// releaseNotes turns commit subjects (newest first, as git log prints them)
// into the notes: new things first, then speed-ups, then fixes, oldest first
// within each, one per line.
func releaseNotes(subjects []string) string {
	byKind := map[string][]string{}
	seen := map[string]bool{}
	for i := len(subjects) - 1; i >= 0; i-- {
		m := commitSubjectRe.FindStringSubmatch(strings.TrimSpace(subjects[i]))
		if m == nil {
			continue
		}
		summary := strings.TrimSpace(m[2])
		if summary == "" || seen[strings.ToLower(summary)] {
			continue
		}
		seen[strings.ToLower(summary)] = true
		byKind[m[1]] = append(byKind[m[1]], strings.ToUpper(summary[:1])+summary[1:])
	}
	var lines []string
	for _, k := range noteKinds {
		for _, s := range byKind[k.kind] {
			lines = append(lines, "• "+k.label+": "+s)
		}
	}
	if len(lines) > maxNoteLines {
		more := len(lines) - maxNoteLines
		lines = append(lines[:maxNoteLines], fmt.Sprintf("…and %d more", more))
	}
	return strings.Join(lines, "\n")
}

// parseVersion reads "1.2.3" (or "1.2.3.4"); ok is false for anything else.
func parseVersion(v string) ([]int, bool) {
	parts := strings.Split(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".")
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out[i] = n
	}
	return out, len(out) >= 3
}

// versionLess says whether a is an earlier version than b.
func versionLess(a, b []int) bool {
	for i := 0; i < len(a) || i < len(b); i++ {
		var x, y int
		if i < len(a) {
			x = a[i]
		}
		if i < len(b) {
			y = b[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

// previousRelease is the newest version of the flavor below version among
// what kashi has published, or "" when there's none.
func previousRelease(releases []publishedRelease, flavor, version string) string {
	target, ok := parseVersion(version)
	if !ok {
		return ""
	}
	best, bestV := "", []int(nil)
	for _, r := range releases {
		v, ok := parseVersion(r.Version)
		if r.Flavor != flavor || !ok || !versionLess(v, target) {
			continue
		}
		if bestV == nil || versionLess(bestV, v) {
			best, bestV = r.Version, v
		}
	}
	return best
}

func gitOutput(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	return strings.TrimSpace(string(out)), err
}

func tagExists(tag string) bool {
	_, err := gitOutput("rev-parse", "--verify", "--quiet", "refs/tags/"+tag)
	return err == nil
}

// autoNotes writes the notes for version from the commits since the last
// published build of the flavor (or the tag before, when kashi has none or
// its tag isn't here). It says what it based them on; "" when there's
// nothing to say or git can't tell.
func autoNotes(baseURL, token, flavor, version string) (notes, since string) {
	var rels struct {
		Releases []publishedRelease `json:"releases"`
	}
	if err := apiJSON(http.MethodGet, baseURL+"/app_releases", token, nil, &rels); err != nil {
		fmt.Printf("Couldn't list kashi's releases for the notes (%v); using the previous tag.\n", err)
	}
	to := "HEAD"
	if tagExists("v" + version) {
		to = "v" + version
	}
	from := ""
	if prev := previousRelease(rels.Releases, flavor, version); prev != "" && tagExists("v"+prev) {
		from = "v" + prev
	} else if tag, err := gitOutput("describe", "--tags", "--abbrev=0", to+"^"); err == nil {
		from = tag
	}
	args := []string{"log", "--no-merges", "--format=%s"}
	if from != "" {
		args = append(args, from+".."+to)
	} else {
		args = append(args, "-n", "30", to)
	}
	out, err := gitOutput(args...)
	if err != nil || out == "" {
		return "", from
	}
	return releaseNotes(strings.Split(out, "\n")), from
}
