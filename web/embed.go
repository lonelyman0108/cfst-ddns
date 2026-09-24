// Package web 嵌入前端构建产物。先在本目录执行 pnpm build，再编译 Go 程序。
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// Dist 返回前端静态资源；未构建时（仅有 .gitkeep）返回 nil。
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}
