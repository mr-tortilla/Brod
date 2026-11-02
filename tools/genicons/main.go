//go:build ignore

// Генератор иконок Brod: go run tools/genicons/main.go
//
// Рисует букву B (вертикальный штрих + две чаши) и пишет:
//   - build/appicon.png            — иконка приложения 1024px (синяя плашка, белая B)
//   - build/windows/icon.ico       — она же для Windows, 16–256px
//   - build/linux/icons/NxN.png    — она же для темы hicolor в Linux-пакетах
//   - internal/tray/icons/*.png|ico — B для трея: серая, зелёная, красная
//
// Та же геометрия продублирована в SVG-логотипе интерфейса (frontend/src/components/Logo.vue).
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/image/vector"
)

const k = 0.5523 // коэффициент кубической аппроксимации четверти окружности

var (
	accent = color.RGBA{0x4f, 0x8c, 0xff, 0xff}
	white  = color.RGBA{0xff, 0xff, 0xff, 0xff}

	trayColors = map[string]color.RGBA{
		"off":  {0x9a, 0xa3, 0xb5, 0xff},
		"on":   {0x3e, 0xcf, 0x8e, 0xff},
		"lost": {0xff, 0x5d, 0x5d, 0xff},
	}
)

// pen переводит координаты сетки 100×100 в пиксели.
type pen struct {
	r      *vector.Rasterizer
	scale  float32
	dx, dy float32
}

func (p pen) pt(x, y float32) (float32, float32) { return p.dx + x*p.scale, p.dy + y*p.scale }
func (p pen) move(x, y float32)                  { p.r.MoveTo(p.pt(x, y)) }
func (p pen) line(x, y float32)                  { p.r.LineTo(p.pt(x, y)) }
func (p pen) cube(x1, y1, x2, y2, x, y float32) {
	ax, ay := p.pt(x1, y1)
	bx, by := p.pt(x2, y2)
	cx, cy := p.pt(x, y)
	p.r.CubeTo(ax, ay, bx, by, cx, cy)
}

// arcDown — правая полуокружность сверху вниз, arcUp — снизу вверх.
func (p pen) arcDown(cx, cy, r float32) {
	p.cube(cx+k*r, cy-r, cx+r, cy-k*r, cx+r, cy)
	p.cube(cx+r, cy+k*r, cx+k*r, cy+r, cx, cy+r)
}

func (p pen) arcUp(cx, cy, r float32) {
	p.cube(cx+k*r, cy+r, cx+r, cy+k*r, cx+r, cy)
	p.cube(cx+r, cy-k*r, cx+k*r, cy-r, cx, cy-r)
}

// ring — «D»-образная чаша: внешний контур по часовой, отверстие — против.
func (p pen) ring(left, top, bottom, cx, w float32) {
	cy, r := (top+bottom)/2, (bottom-top)/2
	p.move(left, top)
	p.line(cx, top)
	p.arcDown(cx, cy, r)
	p.line(left, bottom)
	p.r.ClosePath()

	p.move(left+w, top+w)
	p.line(left+w, bottom-w)
	p.line(cx, bottom-w)
	p.arcUp(cx, cy, r-w)
	p.r.ClosePath()
}

// letterB рисует B в сетке 100×100: занимает x 20..83, y 14..86.
func letterB(p pen) {
	const w = 12
	// штрих
	p.move(20, 14)
	p.line(34, 14)
	p.line(34, 86)
	p.line(20, 86)
	p.r.ClosePath()
	p.ring(27, 14, 50, 55, w) // верхняя чаша
	p.ring(27, 38, 86, 59, w) // нижняя чаша, шире
}

func roundRect(p pen, x0, y0, x1, y1, r float32) {
	p.move(x0+r, y0)
	p.line(x1-r, y0)
	p.cube(x1-r+k*r, y0, x1, y0+r-k*r, x1, y0+r)
	p.line(x1, y1-r)
	p.cube(x1, y1-r+k*r, x1-r+k*r, y1, x1-r, y1)
	p.line(x0+r, y1)
	p.cube(x0+r-k*r, y1, x0, y1-r+k*r, x0, y1-r)
	p.line(x0, y0+r)
	p.cube(x0, y0+r-k*r, x0+r-k*r, y0, x0+r, y0)
	p.r.ClosePath()
}

func fill(dst *image.RGBA, c color.RGBA, draw func(p pen), scale, dx, dy float32) {
	size := dst.Bounds().Dx()
	r := vector.NewRasterizer(size, size)
	draw(pen{r: r, scale: scale, dx: dx, dy: dy})
	r.Draw(dst, dst.Bounds(), image.NewUniform(c), image.Point{})
}

// appIcon — синяя скруглённая плашка с белой B.
func appIcon(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float32(size) / 100
	fill(img, accent, func(p pen) { roundRect(p, 6, 6, 94, 94, 20) }, s, 0, 0)
	// B масштабируем до ~60% и центрируем: центр буквы (51.5, 50) → (50, 50)
	const bs = 0.6
	fill(img, white, letterB, s*bs, (50-51.5*bs)*s, (50-50*bs)*s)
	return img
}

// trayIcon — только буква нужного цвета на прозрачном фоне, почти во весь размер.
func trayIcon(size int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	s := float32(size) / 100
	const bs = 1.3 // буква занимает 63×72 сетки → растягиваем почти на всю иконку
	fill(img, c, letterB, s*bs, (50-51.5*bs)*s, (50-50*bs)*s)
	return img
}

func pngBytes(img image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		log.Fatal(err)
	}
	return b.Bytes()
}

// ico собирает ICO с PNG внутри (поддерживается с Windows Vista).
func ico(images map[int][]byte, sizes []int) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	_ = binary.Write(&b, le, [3]uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for _, sz := range sizes {
		dim := uint8(sz)
		if sz >= 256 {
			dim = 0
		}
		_ = binary.Write(&b, le, struct {
			W, H, Colors, Reserved uint8
			Planes, BPP            uint16
			Size, Offset           uint32
		}{dim, dim, 0, 0, 1, 32, uint32(len(images[sz])), uint32(offset)})
		offset += len(images[sz])
	}
	for _, sz := range sizes {
		b.Write(images[sz])
	}
	return b.Bytes()
}

func write(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("%s (%d байт)", path, len(data))
}

func main() {
	write("build/appicon.png", pngBytes(appIcon(1024)))

	appSizes := []int{16, 24, 32, 48, 64, 128, 256}
	appPNG := map[int][]byte{}
	for _, sz := range appSizes {
		appPNG[sz] = pngBytes(appIcon(sz))
	}
	write("build/windows/icon.ico", ico(appPNG, appSizes))

	for _, sz := range []int{16, 32, 48, 64, 128, 256, 512} {
		write(fmt.Sprintf("build/linux/icons/%dx%d.png", sz, sz), pngBytes(appIcon(sz)))
	}

	traySizes := []int{16, 24, 32, 64}
	for name, c := range trayColors {
		write("internal/tray/icons/"+name+".png", pngBytes(trayIcon(64, c)))
		m := map[int][]byte{}
		for _, sz := range traySizes {
			m[sz] = pngBytes(trayIcon(sz, c))
		}
		write("internal/tray/icons/"+name+".ico", ico(m, traySizes))
	}
}
