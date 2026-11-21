package front

import (
	"embed"
	"io/fs"
)

//go:embed templates/*.html templates/component/*.html assets
var content embed.FS

func Templates() fs.FS {
	return content
}

func Assets() (fs.FS, error) {
	return fs.Sub(content, "assets")
}
