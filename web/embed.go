// Package web holds the GUI built into web/dist by make web.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist is the built GUI, with the files of web/dist at its root. Without
// make web it holds only .gitkeep.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist is embedded; Sub fails only on a bad name
	}
	return sub
}
