// Package webui embeds the built single-page application so that a single
// mybox binary can serve both the API and the web UI.
//
// The web/dist output is copied here by `make web-build`. Only a placeholder
// .gitkeep (under dist/) is committed; the real index.html and assets are
// gitignored build output, so running without a prior `make web-build` embeds
// an empty dist and the UI will be unavailable.
package webui

import "embed"

//go:embed all:dist
var FS embed.FS
