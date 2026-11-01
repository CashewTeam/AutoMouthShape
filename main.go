package main

import (
	"automouthshape/internal/desktop"
	"flag"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"image/png"
	"log"
	"os"
	"path/filepath"
)

func main() {
	root := flag.String("runtime", "", "离线分析资源目录")
	shot := flag.String("screenshot", "", "将初始原生界面渲染为 PNG 后退出")
	flag.Parse()
	if *root == "" {
		base, e := mygo.App.Path(mygo.PathResources)
		if e != nil {
			log.Fatal(e)
		}
		*root = filepath.Join(base, "runtime")
	}
	abs, e := filepath.Abs(*root)
	if e != nil {
		log.Fatal(e)
	}
	app, e := desktop.New(abs)
	if e != nil {
		log.Fatal(e)
	}
	if *shot != "" {
		f, e := os.Create(*shot)
		if e != nil {
			log.Fatal(e)
		}
		e = png.Encode(f, ui.Render(app.View, 1440, 940, 1))
		f.Close()
		if e != nil {
			log.Fatal(e)
		}
		return
	}
	if e = app.Run(); e != nil {
		log.Fatal(e)
	}
}
