// Package web provides embedded static assets and HTML templates.
package web

import "embed"

// TemplateFS holds all HTML template files.
//
//go:embed templates/*.html
var TemplateFS embed.FS

// StaticFS holds all static assets (CSS, JS, images).
//
//go:embed static/*
var StaticFS embed.FS
