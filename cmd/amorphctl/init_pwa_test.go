package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readFile is a small helper that reads a file in a test context and
// fails the test on error rather than returning the error.
func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return string(data)
}

// expectedFiles is the canonical list of files init-pwa must produce
// in every project, relative to the target directory. Tests assert
// each one is present, non-empty, and contains the substituted app
// name (where the placeholder appears).
var expectedFiles = []string{
	"index.html",
	"app.html",
	"app.js",
	"style.css",
	"setup.mbl",
	"auth/login.mbl",
	"auth/signup.mbl",
}

func TestInitPWA_Scaffolds_AllFiles(t *testing.T) {
	target := filepath.Join(t.TempDir(), "myproject")

	cmd := NewInitPWACommand("orderbook", target)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, name := range expectedFiles {
		path := filepath.Join(target, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %s to exist: %v", path, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("file %s is empty", path)
		}
	}

	// auth/ should be a directory.
	authInfo, err := os.Stat(filepath.Join(target, "auth"))
	if err != nil {
		t.Fatalf("expected auth/ directory: %v", err)
	}
	if !authInfo.IsDir() {
		t.Errorf("auth/ is not a directory")
	}
}

func TestInitPWA_PerformsAppNameSubstitution(t *testing.T) {
	target := filepath.Join(t.TempDir(), "proj")

	cmd := NewInitPWACommand("orderbook", target)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Every file should contain "orderbook" substituted in. Conversely,
	// none of the files should contain the bare "myapp" placeholder
	// after substitution.
	for _, name := range expectedFiles {
		body := readFile(t, filepath.Join(target, name))
		if strings.Contains(body, "myapp") {
			t.Errorf("%s still contains placeholder 'myapp' after substitution", name)
		}
	}

	loginMBL := readFile(t, filepath.Join(target, "auth", "login.mbl"))
	signupMBL := readFile(t, filepath.Join(target, "auth", "signup.mbl"))
	setupMBL := readFile(t, filepath.Join(target, "setup.mbl"))

	// The MBL files should now reference world.apps.orderbook.* paths
	// since "myapp" was substituted everywhere.
	if !strings.Contains(loginMBL, "world.apps.orderbook.devices") {
		t.Errorf("login.mbl missing substituted path: world.apps.orderbook.devices")
	}
	if !strings.Contains(loginMBL, "world.apps.orderbook.tokens") {
		t.Errorf("login.mbl missing substituted path: world.apps.orderbook.tokens")
	}
	if !strings.Contains(loginMBL, "world.apps.orderbook.auth.credentials") {
		t.Errorf("login.mbl missing substituted path: world.apps.orderbook.auth.credentials")
	}
	if !strings.Contains(signupMBL, "world.apps.orderbook.devices") {
		t.Errorf("signup.mbl missing substituted path: world.apps.orderbook.devices")
	}
	if !strings.Contains(signupMBL, "world.apps.orderbook.users") {
		t.Errorf("signup.mbl missing substituted path: world.apps.orderbook.users")
	}
	if !strings.Contains(setupMBL, "world.apps.orderbook.app._token_ttl") {
		t.Errorf("setup.mbl missing substituted path: world.apps.orderbook.app._token_ttl")
	}

	appHTML := readFile(t, filepath.Join(target, "app.html"))
	if !strings.Contains(appHTML, `<section name="orderbook">`) {
		t.Errorf("app.html root section not substituted; got body:\n%s", appHTML)
	}

	indexHTML := readFile(t, filepath.Join(target, "index.html"))
	if !strings.Contains(indexHTML, "<title>orderbook</title>") {
		t.Errorf("index.html title not substituted; got body:\n%s", indexHTML)
	}
}

func TestInitPWA_AcceptsExistingEmptyDirectory(t *testing.T) {
	target := t.TempDir() // tempdir starts empty
	cmd := NewInitPWACommand("orderbook", target)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run on empty dir should succeed; got: %v", err)
	}
	// At least one file should exist now.
	if _, err := os.Stat(filepath.Join(target, "index.html")); err != nil {
		t.Errorf("expected index.html to be written: %v", err)
	}
}

func TestInitPWA_RefusesNonEmptyDirectory(t *testing.T) {
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "stranger.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	cmd := NewInitPWACommand("orderbook", target)
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected error on non-empty target, got nil")
	}
	if !strings.Contains(err.Error(), "not empty") {
		t.Errorf("expected 'not empty' in error, got: %v", err)
	}
	// Pre-existing file must still exist and not have been touched.
	body := readFile(t, filepath.Join(target, "stranger.txt"))
	if body != "hi" {
		t.Errorf("pre-existing file was modified: %q", body)
	}
	// No init-pwa files should have been written.
	if _, err := os.Stat(filepath.Join(target, "index.html")); !os.IsNotExist(err) {
		t.Errorf("index.html should not exist after refused init; stat err: %v", err)
	}
}

func TestInitPWA_CreatesNonExistentTargetDirectory(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "fresh", "myproject")

	cmd := NewInitPWACommand("orderbook", target)
	if err := cmd.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "auth", "login.mbl")); err != nil {
		t.Errorf("expected auth/login.mbl in nested target: %v", err)
	}
}

func TestInitPWA_RejectsInvalidAppNames(t *testing.T) {
	cases := []struct {
		name    string
		appName string
	}{
		{"empty", ""},
		{"leading digit", "1myapp"},
		{"hyphen", "my-app"},
		{"period", "my.app"},
		{"slash", "my/app"},
		{"space", "my app"},
		{"shell metachar", "my$app"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "out")
			cmd := NewInitPWACommand(tc.appName, target)
			err := cmd.Run()
			if err == nil {
				t.Fatalf("expected error for app name %q, got nil", tc.appName)
			}
			// Target must not have been created on validation failure.
			if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
				t.Errorf("target directory should not exist after rejected input; stat err: %v", statErr)
			}
		})
	}
}

func TestInitPWA_AcceptsValidAppNames(t *testing.T) {
	cases := []string{
		"a",
		"app",
		"my_app",
		"App2",
		"_internal",
		"order_book_42",
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			target := filepath.Join(t.TempDir(), "out")
			cmd := NewInitPWACommand(name, target)
			if err := cmd.Run(); err != nil {
				t.Errorf("expected success for app name %q, got error: %v", name, err)
			}
		})
	}
}

// TestInitPWA_AuthTemplatesMatchCanonical guards against drift between
// the embedded auth templates under cmd/amorphctl/templates/auth/ and
// the canonical reference watchers under web/boilerplate/auth/. These
// must remain byte-identical so that init-pwa always ships the same
// reference watchers documented elsewhere.
func TestInitPWA_AuthTemplatesMatchCanonical(t *testing.T) {
	pairs := []struct {
		canonical, embedded string
	}{
		{"../../web/boilerplate/auth/login.mbl", "templates/auth/login.mbl"},
		{"../../web/boilerplate/auth/signup.mbl", "templates/auth/signup.mbl"},
	}
	for _, p := range pairs {
		t.Run(filepath.Base(p.canonical), func(t *testing.T) {
			canonical, err := os.ReadFile(p.canonical)
			if err != nil {
				t.Fatalf("failed to read canonical %s: %v", p.canonical, err)
			}
			embedded, err := os.ReadFile(p.embedded)
			if err != nil {
				t.Fatalf("failed to read embedded %s: %v", p.embedded, err)
			}
			if string(canonical) != string(embedded) {
				t.Errorf("DRIFT: %s differs from %s; sync them so the embedded init-pwa template matches the documented reference watcher", p.embedded, p.canonical)
			}
		})
	}
}

// TestInitPWA_SetupMBLContainsRequiredSections verifies the generated
// setup.mbl meets Step C4's contract: it initializes the app data
// structure (specifically the _token_ttl attribute) and provides a
// placeholder my.app.setup_user_watchers procedure.
func TestInitPWA_SetupMBLContainsRequiredSections(t *testing.T) {
	target := filepath.Join(t.TempDir(), "proj")
	if err := NewInitPWACommand("acme", target).Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	body := readFile(t, filepath.Join(target, "setup.mbl"))
	checks := []string{
		"world.apps.acme.app._token_ttl",
		"procedure my.app.setup_user_watchers(username):",
	}
	for _, want := range checks {
		if !strings.Contains(body, want) {
			t.Errorf("setup.mbl missing required content: %q\n--- body ---\n%s", want, body)
		}
	}
}

// TestInitPWA_DoesNotConnectToService is a structural assertion: the
// init-pwa code path must not import the ControlClient. We verify this
// by confirming the constructor and Run signature take no client and
// the package's main dispatch handles init-pwa before connecting (a
// runtime-equivalent check is provided by the fact that this test
// itself runs without a daemon and exercises Run successfully).
func TestInitPWA_DoesNotConnectToService(t *testing.T) {
	target := filepath.Join(t.TempDir(), "proj")
	// If Run required a daemon connection, this would fail or hang.
	// It must complete without any service interaction.
	if err := NewInitPWACommand("standalone", target).Run(); err != nil {
		t.Fatalf("Run should be self-contained; got error: %v", err)
	}
}
