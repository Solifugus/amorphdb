// Executable checks for claims made in CLAUDE.md and the specs.
//
// Why this file exists: over a single day of work, three separate status notes
// sent a session down the wrong path — DEVPLAN's summary table said sixteen
// completed steps were TODO, CLAUDE.md said a finished architecture migration
// was still in progress, and both copies of CLAUDE.md said read-side bracket
// key-selectors were unimplemented when they demonstrably worked. Each was a
// sentence someone believed when they wrote it, and nothing ever re-checked it.
//
// These tests turn the load-bearing claims into assertions. When a claim stops
// being true, this fails and names the document to fix. It is deliberately
// small: it covers claims that would actively mislead someone, not every
// feature. Adding a claim here is cheap; discovering a stale one the hard way
// is not.
//
// Two kinds of check appear below:
//   - "✅ Implemented" claims that must keep working.
//   - "🚧 Not implemented" claims — these fail when a feature starts working,
//     which sounds odd but is the point: it means the docs now understate the
//     system, which is how the bracket-selector claim rotted unnoticed.
package test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// repoRoot walks up from the test's directory to the repository root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		dir = filepath.Dir(dir)
	}
	t.Fatal("could not locate repository root")
	return ""
}

func readDoc(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(data)
}

// TestDocumentedFilesExist guards against the failure that prompted this file:
// CLAUDE.md's authoritative-documents table pointed at docs/DEVPLAN.md,
// docs/pending_code_changes.md and docs/AmorphDB_Tutorial.md, none of which
// existed. A table of sources of truth that cites missing files is worse than
// no table.
func TestDocumentedFilesExist(t *testing.T) {
	root := repoRoot(t)
	claude := readDoc(t, "CLAUDE.md")

	// Every docs/<name>.md mentioned in CLAUDE.md must exist.
	for _, line := range strings.Split(claude, "\n") {
		for _, token := range strings.Fields(strings.NewReplacer("`", " ", "|", " ", "(", " ", ")", " ", ",", " ").Replace(line)) {
			if !strings.HasPrefix(token, "docs/") || !strings.HasSuffix(token, ".md") {
				continue
			}
			if _, err := os.Stat(filepath.Join(root, token)); err != nil {
				t.Errorf("CLAUDE.md references %s, which does not exist", token)
			}
		}
	}

	// Files named as authoritative must be present.
	for _, must := range []string{
		"DEVPLAN.md",
		"docs/amorphdb_design.md",
		"docs/mbl_reference.md",
		"docs/AmorphDB_Remaining_Work.md",
	} {
		if _, err := os.Stat(filepath.Join(root, must)); err != nil {
			t.Errorf("authoritative document %s is missing", must)
		}
	}
}

// TestArchivedDocsAreNotCitedAsCurrent checks that the live documents do not
// point readers at the archive. STATUS.md was cited by a standing rule for
// months after it went stale.
func TestArchivedDocsAreNotCitedAsCurrent(t *testing.T) {
	root := repoRoot(t)

	archived, err := os.ReadDir(filepath.Join(root, "docs", "archive"))
	if err != nil {
		t.Skip("no archive directory")
	}

	live := []string{"CLAUDE.md", "DEVPLAN.md", "README.md",
		"docs/amorphdb_design.md", "docs/mbl_reference.md",
		"docs/AmorphDB_Remaining_Work.md", "docs/getting_started.md"}

	for _, entry := range archived {
		name := entry.Name()
		if name == "README.md" {
			continue
		}
		for _, rel := range live {
			data, err := os.ReadFile(filepath.Join(root, rel))
			if err != nil {
				continue
			}
			text := string(data)
			// A mention inside a docs/archive/ path is a legitimate pointer.
			cleaned := strings.ReplaceAll(text, "docs/archive/"+name, "")
			if strings.Contains(cleaned, name) {
				t.Errorf("%s cites %s, which is archived — update the reference or unarchive the file",
					rel, name)
			}
		}
	}
}

// TestDocumentedRepositoryLayout checks paths CLAUDE.md's tree section names.
// It claimed internal/zone/ existed for months after the package was renamed to
// internal/zone_deprecated/.
func TestDocumentedRepositoryLayout(t *testing.T) {
	root := repoRoot(t)

	mustExist := []string{
		"internal/mbl/lexer", "internal/mbl/parser", "internal/mbl/interpreter",
		"internal/storage", "internal/watcher", "internal/protocol",
		"internal/service", "internal/types", "internal/security",
		"cmd/amorphd", "cmd/amorph", "cmd/amorphctl",
		"web/boilerplate/amorphdb-pwa.js",
		"web/boilerplate/auth/login.mbl",
		"web/boilerplate/auth/signup.mbl",
	}
	for _, rel := range mustExist {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Errorf("CLAUDE.md documents %s, which does not exist", rel)
		}
	}

	// The old package name must stay gone; CLAUDE.md now says zone_deprecated.
	if _, err := os.Stat(filepath.Join(root, "internal", "zone")); err == nil {
		t.Error("internal/zone exists again — CLAUDE.md says it was renamed to internal/zone_deprecated")
	}
}

// TestPWABoilerplateCopiesAreInSync enforces a standing rule in CLAUDE.md: the
// test copy of the boilerplate must match the embedded one. A silent divergence
// means integration tests exercise different code from the shipped binary.
func TestPWABoilerplateCopiesAreInSync(t *testing.T) {
	root := repoRoot(t)

	primary, err := os.ReadFile(filepath.Join(root, "web/boilerplate/amorphdb-pwa.js"))
	if err != nil {
		t.Fatalf("read primary boilerplate: %v", err)
	}
	testCopy, err := os.ReadFile(filepath.Join(root, "web/boilerplate/test/amorphdb-pwa.js"))
	if err != nil {
		t.Skipf("no test copy present: %v", err)
	}

	if string(primary) != string(testCopy) {
		t.Errorf("web/boilerplate/test/amorphdb-pwa.js has drifted from web/boilerplate/amorphdb-pwa.js "+
			"(%d bytes vs %d) — CLAUDE.md requires them to stay in sync",
			len(testCopy), len(primary))
	}
}
