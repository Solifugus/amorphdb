// Package main provides the init-pwa scaffolding command for amorphctl.
//
// `amorphctl init-pwa <appname> <directory>` writes a starter PWA
// project into the target directory. It is a local file-system
// operation and does not require a connection to the amorphd service.
//
// Files written:
//
//	<directory>/index.html        — loader (HTML)
//	<directory>/app.html          — starter section template
//	<directory>/app.js            — empty browser-side helpers
//	<directory>/style.css         — minimal default styles
//	<directory>/setup.mbl         — app data structure init + setup_user_watchers
//	<directory>/auth/login.mbl    — reference login watcher (Step C1)
//	<directory>/auth/signup.mbl   — reference signup watcher (Step C2)
//
// Each template contains the placeholder `myapp` which is replaced
// with the supplied app name during scaffolding. The placeholder
// appears in the watcher path roots (`world.apps.myapp.*`), the root
// section name in `app.html`, the page title in `index.html`, and
// header comments throughout. The reference auth watcher templates
// embedded here are byte-identical to the canonical copies under
// `web/boilerplate/auth/`; a drift test in init_pwa_test.go enforces
// this.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates
var pwaTemplates embed.FS

// appNamePlaceholder is the literal substring replaced with the
// project's chosen app name in every template file.
const appNamePlaceholder = "myapp"

// templateRoot is the embedded directory containing all init-pwa
// templates. It mirrors the on-disk project layout one-for-one — the
// scaffolder simply walks the embedded tree and writes each file to
// the target directory after substitution.
const templateRoot = "templates"

// InitPWACommand scaffolds a new PWA project on disk.
type InitPWACommand struct {
	appName   string
	targetDir string
}

// NewInitPWACommand creates a new init-pwa command handler.
func NewInitPWACommand(appName, targetDir string) *InitPWACommand {
	return &InitPWACommand{appName: appName, targetDir: targetDir}
}

// Run validates inputs, creates the target directory if needed, and
// writes every embedded template into it after replacing the
// `myapp` placeholder with the project's app name. Refuses to
// overwrite an existing non-empty directory.
func (c *InitPWACommand) Run() error {
	if err := validateAppName(c.appName); err != nil {
		return err
	}

	nonEmpty, err := dirIsNonEmpty(c.targetDir)
	if err != nil {
		return err
	}
	if nonEmpty {
		return fmt.Errorf("target directory %q is not empty; refusing to overwrite", c.targetDir)
	}

	if err := os.MkdirAll(c.targetDir, 0o755); err != nil {
		return fmt.Errorf("failed to create target directory %s: %w", c.targetDir, err)
	}

	written := 0
	walkErr := fs.WalkDir(pwaTemplates, templateRoot, func(srcPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if srcPath == templateRoot {
			return nil
		}
		// Path within the project, relative to the target directory.
		rel := strings.TrimPrefix(srcPath, templateRoot+"/")
		dstPath := filepath.Join(c.targetDir, rel)

		if d.IsDir() {
			if err := os.MkdirAll(dstPath, 0o755); err != nil {
				return fmt.Errorf("failed to create %s: %w", dstPath, err)
			}
			return nil
		}

		data, err := pwaTemplates.ReadFile(srcPath)
		if err != nil {
			return fmt.Errorf("failed to read embedded template %s: %w", srcPath, err)
		}
		substituted := strings.ReplaceAll(string(data), appNamePlaceholder, c.appName)
		if err := os.WriteFile(dstPath, []byte(substituted), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", dstPath, err)
		}
		written++
		return nil
	})
	if walkErr != nil {
		return walkErr
	}

	fmt.Printf("✓ Scaffolded PWA project '%s' (%d files) in %s\n", c.appName, written, c.targetDir)
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit app.html to define your sections.\n")
	fmt.Printf("  2. Edit setup.mbl to register per-user watchers.\n")
	fmt.Printf("  3. Deploy with: amorphctl deploy-pwa %s.<host> %s\n", c.appName, c.targetDir)
	return nil
}

// validateAppName enforces that the app name is a valid MBL
// identifier. The name appears in MBL paths (world.apps.<name>.*),
// HTML section attributes, and CSS-class-like contexts, so the
// validation matches the existing isValidMBLIdentifier check used by
// extract.
func validateAppName(name string) error {
	if name == "" {
		return fmt.Errorf("app name cannot be empty")
	}
	if !isValidMBLIdentifier(name) {
		return fmt.Errorf("app name %q must be a valid identifier (letters, digits, underscores; cannot start with a digit)", name)
	}
	return nil
}

// dirIsNonEmpty reports whether the directory at path exists and
// contains at least one entry. A non-existent directory returns
// (false, nil) so the caller can create it; an existing empty
// directory also returns (false, nil) so the caller can populate it.
func dirIsNonEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to inspect %s: %w", path, err)
	}
	return len(entries) > 0, nil
}
