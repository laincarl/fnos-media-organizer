// Package web embeds the browser interface into the service binary.
package web

import "embed"

// Assets contains only the files served by the organizer UI.
//
//go:embed index.html app.js style.css app-icon.png chevron.svg
var Assets embed.FS
