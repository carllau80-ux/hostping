// syso 生成 Windows 资源对象文件：图标写入资源 ID 1/2/3（Wails 窗口读取 ID 3）+ 清单。
// 用法: go run ./devtools/syso -ico build/icon.ico -o rsrc_windows_amd64.syso
package main

import (
	"flag"
	"os"

	"github.com/tc-hib/winres"
)

func main() {
	icoPath := flag.String("ico", "build/icon.ico", "图标文件")
	outPath := flag.String("o", "rsrc_windows_amd64.syso", "输出 .syso")
	flag.Parse()

	f, err := os.Open(*icoPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	icon, err := winres.LoadICO(f)
	if err != nil {
		panic(err)
	}

	rs := &winres.ResourceSet{}
	// Wails 在窗口创建时按资源 ID 3（AppIconID）加载图标，缺省则回退系统默认图标。
	// 同时写入 1/2 保证资源管理器等场景使用。
	for _, id := range []uint16{1, 2, 3} {
		if err := rs.SetIcon(winres.ID(id), icon); err != nil {
			panic(err)
		}
	}
	rs.SetManifest(winres.AppManifest{})

	out, err := os.Create(*outPath)
	if err != nil {
		panic(err)
	}
	defer out.Close()
	if err := rs.WriteObject(out, winres.ArchAMD64); err != nil {
		panic(err)
	}
	println("written:", *outPath)
}
