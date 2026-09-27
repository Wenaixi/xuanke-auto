//go:build windows

package main

import (
	"encoding/binary"
	"testing"
)

// 托盘 ICO 资产回归钉：历史上两次手写字节失位（1x1 全透明 / dwBytesInRes 与
// dwImageOffset 写反）后，用机器检查固化 ICO 结构合法性与像素语义——未来任何换图
// 只要结构或像素破坏即 FAIL，绝不静默产出坏图标。Windows 托盘链路
// （systray → LoadImageW → DrawIconEx）与本断言同口径。
func TestTrayIconAsset(t *testing.T) {
	ico := trayIcon()
	if len(ico) < 6 {
		t.Fatalf("ICO 过短: %d 字节", len(ico))
	}
	// ICONDIR：reserved=0 / type=1(icon)
	if ico[0] != 0 || ico[1] != 0 || ico[2] != 1 || ico[3] != 0 {
		t.Fatalf("ICONDIR 头非法: %v", ico[:4])
	}

	// 尺寸序列是契约：16/20/24 覆盖 100%/125%/150% 缩放的托盘尺寸，32/48 供高 DPI。
	want := []int{16, 20, 24, 32, 48}
	count := int(binary.LittleEndian.Uint16(ico[4:6]))
	if count != len(want) {
		t.Fatalf("条目数须 %d, got %d", len(want), count)
	}

	for i, size := range want {
		entry := ico[6+16*i : 6+16*(i+1)]
		if int(entry[0]) != size || int(entry[1]) != size {
			t.Fatalf("条目 %d 尺寸须 %dx%d, got %dx%d", i, size, size, entry[0], entry[1])
		}
		if binary.LittleEndian.Uint16(entry[4:6]) != 1 || binary.LittleEndian.Uint16(entry[6:8]) != 32 {
			t.Fatalf("条目 %d planes/bitcount 非法", i)
		}
		bytesInRes := int(binary.LittleEndian.Uint32(entry[8:12]))
		offset := int(binary.LittleEndian.Uint32(entry[12:16]))
		// 数据区长度按格式语义独立推导（DIB 头 40 + 像素 w*h*4 + AND 掩码逐行 4 字节对齐），
		// 与实现「总长减头部」不同源——字段算错（像素字节数写错/offset 值打进）仍能红。
		andRow := ((size + 31) / 32) * 4
		if wantBytes := 40 + size*size*4 + andRow*size; bytesInRes != wantBytes {
			t.Fatalf("条目 %d dwBytesInRes 须 %d, got %d", i, wantBytes, bytesInRes)
		}
		if offset+bytesInRes > len(ico) {
			t.Fatalf("条目 %d 数据越界: offset=%d size=%d 总长=%d", i, offset, bytesInRes, len(ico))
		}
		// BITMAPINFOHEADER：biSize=40 / 宽=size / XOR+AND 双高 / planes=1 / bitcount=32
		dib := ico[offset:]
		if binary.LittleEndian.Uint32(dib[0:4]) != 40 {
			t.Fatalf("条目 %d biSize 须 40", i)
		}
		if int(binary.LittleEndian.Uint32(dib[4:8])) != size {
			t.Fatalf("条目 %d 宽度须 %d", i, size)
		}
		if int(binary.LittleEndian.Uint32(dib[8:12])) != size*2 {
			t.Fatalf("条目 %d XOR+AND 双高须 %d, got %d", i, size*2, binary.LittleEndian.Uint32(dib[8:12]))
		}
	}

	// 像素语义（取最大条目 = 48px，每像素 BGRA，DIB 自下而上）：
	// ① 四角必须全透明——白色背景已去净，否则深色托盘上会出现白色方块；
	// ② 白色与黑色不透明像素必须同时存在——全透明（曾出现的 1x1 坏图标）或全黑
	//    （丢失猫爪）都会让本断言变红。
	size := want[len(want)-1]
	entry := ico[6+16*(count-1) : 6+16*count]
	pixels := ico[int(binary.LittleEndian.Uint32(entry[12:16]))+40:]
	at := func(x, y int) (r, g, b, a byte) {
		off := (y*size + x) * 4
		return pixels[off+2], pixels[off+1], pixels[off], pixels[off+3]
	}
	for _, corner := range [][2]int{{0, 0}, {size - 1, 0}, {0, size - 1}, {size - 1, size - 1}} {
		if _, _, _, a := at(corner[0], corner[1]); a != 0 {
			t.Fatalf("角点 %v 须全透明, got alpha=%d", corner, a)
		}
	}
	var white, black int
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			r, g, b, a := at(x, y)
			if a != 255 {
				continue
			}
			if r == 255 && g == 255 && b == 255 {
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
