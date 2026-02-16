package static

import "embed"

//go:embed all:html
var HTML embed.FS

//go:embed all:conf
var Conf embed.FS

//go:embed all:db
var DB embed.FS
