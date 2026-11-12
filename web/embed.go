// Package web は、make web が web/dist に作る画面を持つ。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist は、web/dist の中身を根に持つ画面のファイル。make web をしていなければ .gitkeep だけ。
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // dist は埋め込み済みなので、Sub が失敗するのは名前の誤りだけ
	}
	return sub
}
