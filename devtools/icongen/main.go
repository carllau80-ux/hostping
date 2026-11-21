// icongen 纯 Go 绘制 DBPing 应用图标（数据库圆柱 + 绿色探活点），输出多尺寸 .ico。
// 用法: go run ./devtools/icongen -o build/icon.ico
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"

	xdraw "golang.org/x/image/draw"
)

func main() {
	out := flag.String("o", "build/icon.ico", "输出 .ico 路径")
	pngOut := flag.String("png", "", "同时输出 PNG 预览（可选）")
	flag.Parse()

	const S = 256
	base := image.NewNRGBA(image.Rect(0, 0, S, S))
	drawIcon(base)

	if *pngOut != "" {
		f, err := os.Create(*pngOut)
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, base); err != nil {
			panic(err)
		}
		f.Close()
		println("written:", *pngOut)
	}

	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	imgs := map[int]image.Image{S: base}
	for _, sz := range sizes[:len(sizes)-1] {
		dst := image.NewNRGBA(image.Rect(0, 0, sz, sz))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), base, base.Bounds(), xdraw.Over, nil)
		imgs[sz] = dst
	}

	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		panic(err)
	}
	if err := writeICO(*out, sizes, imgs); err != nil {
		panic(err)
	}
	println("written:", *out)

	// 同步一份给运行时窗口图标注入（internal/winicon 通过 go:embed 引用）
	if err := os.MkdirAll("internal/winicon", 0o755); err != nil {
		panic(err)
	}
	if err := writeICO("internal/winicon/icon.ico", sizes, imgs); err != nil {
		panic(err)
	}
	println("written: internal/winicon/icon.ico")
}

// drawIcon 在 256x256 画布上绘制图标。
func drawIcon(img *image.NRGBA) {
	const S = 256
	set := func(x, y int, c color.NRGBA) {
		if x < 0 || y < 0 || x >= S || y >= S {
			return
		}
		img.SetNRGBA(x, y, c)
	}
	// 背景圆角方块：对角渐变 #1d4ed8 -> #06b6d4，圆角 56
	const radius = 56
	c1 := color.NRGBA{R: 0x1d, G: 0x4e, B: 0xd8, A: 0xff}
	c2 := color.NRGBA{R: 0x06, G: 0xb6, B: 0xd4, A: 0xff}
	for y := 0; y < S; y++ {
		for x := 0; x < S; x++ {
			if !inRoundedRect(x, y, S, S, radius) {
				continue
			}
			t := float64(x+y) / float64(2*S-2)
			set(x, y, lerp(c1, c2, t))
		}
	}

	// 白色数据库圆柱（居中）
	const cx, rx = 128, 62
	const topY, botY = 92, 178
	const ry = 22
	white := color.NRGBA{R: 0xf8, G: 0xfa, B: 0xfc, A: 0xff}
	shell := color.NRGBA{R: 0xdb, G: 0xea, B: 0xfe, A: 0xff}
	shell2 := color.NRGBA{R: 0xbf, G: 0xdb, B: 0xfe, A: 0xff}
	for y := topY - ry; y <= botY+ry; y++ {
		for x := cx - rx - 2; x <= cx+rx+2; x++ {
			// 主体：两条椭圆边之间
			inTop := insideEllipse(x, y, cx, topY, rx, ry)
			inBot := insideEllipse(x, y, cx, botY, rx, ry)
			switch {
			case inTop:
				set(x, y, shell) // 顶面
			case inBot:
				set(x, y, shell2) // 底面
			case x >= cx-rx && x <= cx+rx && y > topY && y < botY:
				set(x, y, white) // 柱身
			}
		}
	}
	// 柱身上两条分隔弧线（浅色），模拟盘片
	for _, yy := range []int{120, 148} {
		for x := cx - rx + 2; x <= cx+rx-2; x++ {
			set(x, yy, color.NRGBA{R: 0xe2, G: 0xe8, B: 0xf0, A: 0xff})
			set(x, yy+1, color.NRGBA{R: 0xe2, G: 0xe8, B: 0xf0, A: 0xff})
		}
	}

	// 绿色探活点（右上，白描边）
	const dx, dy, dr = 188, 74, 21
	for y := dy - dr - 4; y <= dy+dr+4; y++ {
		for x := dx - dr - 4; x <= dx+dr+4; x++ {
			d := dist2(x, y, dx, dy)
			if d <= float64(dr*dr) {
				set(x, y, color.NRGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff})
			} else if d <= float64((dr+4)*(dr+4)) {
				set(x, y, color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff})
			}
		}
	}
}

func inRoundedRect(x, y, w, h, r int) bool {
	if x < 0 || y < 0 || x >= w || y >= h {
		return false
	}
	inCorner := func(cx, cy int) bool {
		return dist2(x, y, cx, cy) <= float64(r*r)
	}
	switch {
	case x < r && y < r:
		return inCorner(r, r)
	case x >= w-r && y < r:
		return inCorner(w-r-1, r)
	case x < r && y >= h-r:
		return inCorner(r, h-r-1)
	case x >= w-r && y >= h-r:
		return inCorner(w-r-1, h-r-1)
	}
	return true
}

func insideEllipse(x, y, cx, cy, rx, ry int) bool {
	dx := float64(x-cx) / float64(rx)
	dy := float64(y-cy) / float64(ry)
	return dx*dx+dy*dy <= 1
}

func dist2(x, y, cx, cy int) float64 {
	return float64((x-cx)*(x-cx) + (y-cy)*(y-cy))
}

func lerp(a, b color.NRGBA, t float64) color.NRGBA {
	return color.NRGBA{
		R: uint8(float64(a.R) + (float64(b.R)-float64(a.R))*t),
		G: uint8(float64(a.G) + (float64(b.G)-float64(a.G))*t),
		B: uint8(float64(a.B) + (float64(b.B)-float64(a.B))*t),
		A: 0xff,
	}
}

// writeICO 写出 .ico：小尺寸用 BMP(32bpp)，128/256 用 PNG。
func writeICO(path string, sizes []int, imgs map[int]image.Image) error {
	type entry struct {
		w, h  int
		blob  []byte
		isPNG bool
	}
	var entries []entry
	for _, sz := range sizes {
		var buf bytes.Buffer
		isPNG := sz >= 128
		if isPNG {
			if err := png.Encode(&buf, imgs[sz]); err != nil {
				return err
			}
		} else if err := encodeBMP32(&buf, imgs[sz]); err != nil {
			return err
		}
		entries = append(entries, entry{w: sz, h: sz, blob: buf.Bytes(), isPNG: isPNG})
	}

	var out bytes.Buffer
	binary.Write(&out, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1)) // type icon
	binary.Write(&out, binary.LittleEndian, uint16(len(entries)))

	offset := 6 + 16*len(entries)
	for _, e := range entries {
		b := byte(0)
		if e.w < 256 {
			b = byte(e.w)
		}
		out.WriteByte(b)               // width
		out.WriteByte(b)               // height
		out.WriteByte(0)               // palette
		out.WriteByte(0)               // reserved
		binary.Write(&out, binary.LittleEndian, uint16(1))
		binary.Write(&out, binary.LittleEndian, uint16(32))
		binary.Write(&out, binary.LittleEndian, uint32(len(e.blob)))
		binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(e.blob)
	}
	for _, e := range entries {
		out.Write(e.blob)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// encodeBMP32 ICO 内嵌 32bpp BMP（自下而上 BGRA + 全零 AND 掩码）。
func encodeBMP32(buf *bytes.Buffer, img image.Image) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	// BITMAPINFOHEADER
	hdr := make([]byte, 40)
	binary.LittleEndian.PutUint32(hdr[0:], 40)
	binary.LittleEndian.PutUint32(hdr[4:], uint32(w))
	binary.LittleEndian.PutUint32(hdr[8:], uint32(h*2))
	binary.LittleEndian.PutUint16(hdr[12:], 1)
	binary.LittleEndian.PutUint16(hdr[14:], 32)
	buf.Write(hdr)
	// 像素（BGRA，自下而上）
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, a := img.At(x, y).RGBA()
			if a == 0 {
				buf.WriteByte(0)
				buf.WriteByte(0)
				buf.WriteByte(0)
				buf.WriteByte(0)
				continue
			}
			buf.WriteByte(uint8(bb >> 8))
			buf.WriteByte(uint8(g >> 8))
			buf.WriteByte(uint8(r >> 8))
			buf.WriteByte(uint8(a >> 8))
		}
	}
	// AND 掩码（每行按 32bit 对齐，全 0 = 不透明由 alpha 决定）
	rowBytes := ((w + 31) / 32) * 4
	mask := make([]byte, rowBytes)
	for y := 0; y < h; y++ {
		buf.Write(mask)
	}
	return nil
}
