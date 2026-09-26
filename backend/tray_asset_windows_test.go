//go:build windows

package main

import (
	"encoding/binary"
	"testing"
)

// 托盘 ICO 资产回归钉：两次手写字节失位（1x1 全透明 / IDAT CRC 错误）后，
// 用机器检查固化 ICO 结构合法性与像素语义——未来任何重排/美化字节只要结构或像素
// 破坏即 FAIL，绝不静默产出坏图标。Windows 托盘链路（systray → LoadImageW →
// DrawIconEx）与本断言同口径。
func TestTrayIconAsset(t *testing.T) {
	ico := trayIcon()
	// ICONDIR：reserved=0 / type=1(icon) / count=1
	if ico[0] != 0 || ico[1] != 0 || ico[2] != 1 || ico[3] != 0 || ico[4] != 1 || ico[5] != 0 {
		t.Fatalf("ICONDIR 头非法: %v", ico[:6])
	}
	// ICONDIRENTRY：宽高 32x32
	if ico[6] != 32 || ico[7] != 32 {
		t.Fatalf("ICO 尺寸须 32x32, got %dx%d", ico[6], ico[7])
	}
	// ICONDIRENTRY[14:18] dwBytesInRes 须为数据区全量。按格式语义三加数独立推导
	// （BITMAPINFOHEADER 40 + 像素 32×32×4=4096 + AND mask 32×4=128 = 4264）——
	// 与实现"总长减头部"不同源，字段值算错（像素字节数写错/offset 值打进）仍能红。
	if want := uint32(4264); binary.LittleEndian.Uint32(ico[14:18]) != want {
		t.Fatalf("dwBytesInRes 须 %d, got %d", want, binary.LittleEndian.Uint32(ico[14:18]))
	}
	// ICONDIRENTRY[18:22] dwImageOffset 须指向目录后首个数据字节（= 22），
	// 而非数据区总长——曾把 len(ico)（4286）填入 offset。
	if off := binary.LittleEndian.Uint32(ico[18:22]); off != 22 {
		t.Fatalf("dwImageOffset 须 22, got %d", off)
	}
	// BITMAPINFOHEADER：biSize=40 / 宽 32 / XOR+AND 双高 64 / planes=1 / bitcount=32
	dib := ico[22:]
	if binary.LittleEndian.Uint32(dib[0:4]) != 40 {
		t.Fatalf("BITMAPINFOHEADER biSize 须 40")
	}
	if binary.LittleEndian.Uint32(dib[4:8]) != 32 {
		t.Fatalf("宽度须 32")
	}
	if binary.LittleEndian.Uint32(dib[8:12]) != 64 {
		t.Fatalf("XOR+AND 双高须 64, got %d", binary.LittleEndian.Uint32(dib[8:12]))
	}
	// 像素：pixStart = ICONDIR(6) + ICONDIRENTRY(16) + BITMAPINFOHEADER(40) = 62，
	// 每像素 BGRA 4 字节。中心 4x4（14..17）白不透明、其余纯黑不透明。
	pixStart := 22 + 40
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			off := pixStart + (y*32+x)*4
			b, g, r, a := ico[off], ico[off+1], ico[off+2], ico[off+3]
			center := x >= 14 && x <= 17 && y >= 14 && y <= 17
			if center {
				if r != 255 || g != 255 || b != 255 || a != 255 {
					t.Fatalf("中心(%d,%d) 须白不透明, got BGRA=%v", x, y, []byte{b, g, r, a})
				}
			} else {
				if r != 0 || g != 0 || b != 0 || a != 255 {
					t.Fatalf("非中心(%d,%d) 须黑不透明, got BGRA=%v", x, y, []byte{b, g, r, a})
				}
			}
		}
	}
}
