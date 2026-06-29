// Package boilerplate embeds the AmorphDB PWA client-side framework so the
// amorphd binary can serve it directly at /amorphdb/pwa.js without depending on
// any files on disk at runtime.
//
// The embedded source is amorphdb-pwa.js in this directory. The copy under
// test/ is the test harness's own copy and must be kept in sync with this one
// (see CLAUDE.md), but only this parent copy is embedded and served.
package boilerplate

import _ "embed"

// PWAJavaScript is the full text of the PWA client boilerplate
// (amorphdb-pwa.js), embedded at build time.
//
//go:embed amorphdb-pwa.js
var PWAJavaScript []byte
