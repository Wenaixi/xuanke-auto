//go:build linux && cgo

package main

import (
	"bytes"
	"image/png"
	"io"
	"testing"
)

// 托盘 PNG 资产回归钉：R76/R77 两次手写字节失位（1x1 全透明 / zlib 头换 CRC 错位）后，
// 用 Go 官方 image/png.Decode（严格校验 chunk CRC）固化像素语义——未来任何重排/美化
// 字节只要结构或像素破坏即 FAIL。Linux 托盘链路（systray → AppIndicator）与本断言同口径。
func TestTrayPNGAsset(t *testing.T) {
	img, err := png.Decode(io.Reader(bytes.NewReader(trayPNG())))
	if err != nil {
		t.Fatalf("trayPNG 解码失败（CRC/结构破坏）：%v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 16 || bounds.Dy() != 16 {
		t.Fatalf("尺寸须 16x16, got %dx%d", bounds.Dx(), bounds.Dy())
	}
	// 中心 4x4（6..9）白不透明、其余纯黑不透明。
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			r, g, b, a := img.At(x, y).RGBA()
			// RGBA() 返回 16bit 各分量（0..65535）
			center := x >= 6 && x <= 9 && y >= 6 && y <= 9
			if center {
				if r < 0xFF00 || g < 0xFF00 || b < 0xFF00 || a != 0xFFFF {
					t.Fatalf("中心(%d,%d) 须白不透明, got RGBA=%d,%d,%d,%d", x, y, r, g, b, a)
				}
			} else {
				if r != 0 || g != 0 || b != 0 || a != 0xFFFF {
					t.Fatalf("非中心(%d,%d) 须黑不透明, got RGBA=%d,%d,%d,%d", x, y, r, g, b, a)
				}
			}
		}
	}
}
