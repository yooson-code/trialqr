// Package web menanam template HTML dan aset statis ke dalam binary.
package web

import "embed"

//go:embed templates/*.html static/*
var FS embed.FS
