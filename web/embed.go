package web

import "embed"

//go:embed template/*
var TemplateFS embed.FS

//go:embed static/logo.svg
//go:embed static/closewindow.html
//go:embed static/css/*.min.css
//go:embed static/font/*
//go:embed static/img/*
var StaticFS embed.FS
