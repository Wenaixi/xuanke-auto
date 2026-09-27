//go:build linux && cgo

package main

import (
	"bytes"
	"image/png"
	"io"
	"testing"
)

// 托盘 PNG 资产回归钉：两次手写字节失位（1x1 全透明 / zlib 头换 CRC 错位）后，
// 用 Go 官方 image/png.Decode（严格校验 chunk CRC）固化像素语义——未来任何换图
// 只要结构或像素破坏即 FAIL。Linux 托盘链路（systray → AppIndicator）与本断言同口径。
func TestTrayPNGAsset(t *testing.T) {
	img, err := png.Decode(io.Reader(bytes.NewReader(trayPNG())))
	if err != nil {
		t.Fatalf("trayPNG 解码失败（CRC/结构破坏）：%v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 16 || bounds.Dy() != 16 {
		t.Fatalf("尺寸须 16x16, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	// ① 四角必须全透明——白色背景已去净，否则深色面板上会出现白色方块；
	// ② 白色与黑色不透明像素必须同时存在——全透明或全黑都会让本断言变红。
	for _, corner := range [][2]int{{0, 0}, {15, 0}, {0, 15}, {15, 15}} {
		if _, _, _, a := img.At(corner[0], corner[1]).RGBA(); a != 0 {
			t.Fatalf("角点 %v 须全透明, got alpha=%d", corner, a)
		}
	}
	var white, black int
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			if a != 0xFFFF {
				continue
			}
			if r == 0xFFFF && g == 0xFFFF && b == 0xFFFF {
				white++
			}
			if r == 0 && g == 0 && b == 0 {
				black++
			}
		}
	}
	if white == 0 || black == 0 {
		t.Fatalf("像素语义异常：白 %d / 黑 %d（须同时存在猫爪与方块）", white, black)
	}
}
