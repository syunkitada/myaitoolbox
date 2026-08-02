// Package webui embeds the built single-page application so that a single
// mybox binary can serve both the API and the web UI.
//
// The web/dist output is copied here by `make web-build`. A placeholder
// index.html is committed so that `go build` works without a prior web build.
package webui

import "embed"

//go:embed all:dist
var FS embed.FS
