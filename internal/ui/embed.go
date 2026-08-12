package ui

import "embed"

// staticFS 嵌入前端构建产物（web/ → dist/）。
//
//go:embed all:dist
var staticFS embed.FS
