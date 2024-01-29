package locales

import "embed"

//go:embed active.*.toml
var LocaleFS embed.FS
