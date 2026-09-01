package umc6618

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	Width  = 320
	Height = 240
)

const (
	LayerTilemap0 uint8 = 1 << iota
	LayerTilemap1
	LayerTilemap2
	LayerSprites
	LayerROZ
	LayerWindows
	AllLayers = LayerTilemap0 | LayerTilemap1 | LayerTilemap2 | LayerSprites | LayerROZ | LayerWindows
)

var spriteYSize = [16]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 16, 20, 22, 24, 26}

func (d *Device) vramWord(index uint32) uint16 {
	return d.ReadVRAM16((index & 0xffff) << 1)
}

func (d *Device) tilemapRegion(layer int) int {
	gfxMode := int(d.pixelMode & 7)
	if layer == 0 {
		return [8]int{2, 1, 0, 1, 0, 0, 0, 0}[gfxMode]
	}
	if layer == 1 {
		return [8]int{2, 1, 1, 1, 2, 2, 2, 2}[gfxMode]
	}
	return 2
}

func tilemapDimensions(flags uint16) (int, int) {
	switch flags & 0x0f00 {
	case 0x0200:
		return 16, 16
	case 0x0400:
		return 32, 32
	case 0x0600:
		return 64, 32
	case 0x0a00:
		return 128, 32
	case 0x0c00:
		return 64, 64
	default:
		return 32, 32
	}
}

func (d *Device) tilePixel(region, tile, x, y int) uint8 {
	switch region {
	case 0:
		return d.ReadVRAM8(uint32(tile*64 + y*8 + x))
	case 1:
		value := d.ReadVRAM8(uint32(tile*32 + y*4 + x/2))
		if x&1 != 0 {
			return value >> 4
		}
		return value & 0x0f
	default:
		value := d.ReadVRAM8(uint32(tile*16 + y*2 + x/4))
		return value >> uint((x&3)*2) & 3
	}
}

func (d *Device) tilemapPixel(layer, x, y int) uint16 {
	baseIndex := 0x80 + layer*0x10
	flags := d.registers[baseIndex]
	tileMode := d.registers[baseIndex+1]
	mode := d.registers[baseIndex+5]
	xs, _ := tilemapDimensions(flags)
	region := d.tilemapRegion(layer)
	base := uint32(d.registers[baseIndex+4]) << 1
	entry := d.vramWord(base + uint32((y>>3)*xs+(x>>3)))
	palette := int(entry>>12) & 0x0f
	if tileMode&0x0200 != 0 {
		palette |= 8
	}
	tileBank := int(mode&0x7000) >> 12 << (8 + region)
	tile := int(entry&0x03ff) + tileBank
	px, py := x&7, y&7
	if entry&0x0800 != 0 {
		px ^= 7
	}
	if entry&0x0400 != 0 {
		py ^= 7
	}
	pixel := d.tilePixel(region, tile, px, py)
	if region == 0 {
		return uint16(pixel)
	}
	return uint16(palette*16) + uint16(pixel)
}

func signed12(value uint16) int {
	result := int(value & 0x0fff)
	if result&0x0800 != 0 {
		result -= 0x1000
	}
	return result
}

func (d *Device) drawTilemap(layer, priority int, indexed []uint16, priorities []uint8) {
	baseIndex := 0x80 + layer*0x10
	flags := d.registers[baseIndex]
	tileMode := d.registers[baseIndex+1]
	xs, ys := tilemapDimensions(flags)
	region := d.tilemapRegion(layer)
	transparentMask := uint16(3)
	if region == 0 {
		transparentMask = 0xff
	} else if region == 1 {
		transparentMask = 0x0f
	}
	scrollX, scrollY := signed12(d.registers[baseIndex+2]), signed12(d.registers[baseIndex+3])
	if flags&2 != 0 {
		scrollX ^= xs*8 - 1
	}
	if flags&1 != 0 {
		scrollY ^= ys*8 - 1
	}
	mosaic := int(flags&0x001c) >> 2
	mosaicMask := ^((1 << mosaic) - 1)
	wrap := flags&0x20 != 0
	for y := range Height {
		realY := (y & mosaicMask) + scrollY
		if !wrap && (scrollY+y < 0 || scrollY+y >= ys*8) {
			continue
		}
		if tileMode&0x0800 != 0 {
			line := d.vramWord((uint32(d.registers[baseIndex+7]) << 1) + uint32(y))
			realY = (int(int16(line)) + scrollY) & (ys*8 - 1)
		}
		realY &= ys*8 - 1
		lineScroll := scrollX
		if tileMode&0x4000 != 0 {
			lineScroll += int(int16(d.vramWord((uint32(d.registers[baseIndex+6]) << 1) + uint32(y))))
		}
		for x := range Width {
			realX := (x & mosaicMask) + lineScroll
			if !wrap && (lineScroll+x < 0 || lineScroll+x >= xs*8) {
				continue
			}
			pixel := d.tilemapPixel(layer, realX&(xs*8-1), realY)
			offset := y*Width + x
			if pixel&transparentMask != 0 && priority < int(priorities[offset]>>4) {
				indexed[offset] = pixel
				priorities[offset] = priorities[offset]&0x0f | uint8(priority<<4)
			}
		}
	}
}

func (d *Device) drawSpriteTile(tile, palette int, flipX, flipY bool, destX, destY, priority, maskMode int, sprites []uint16, priorities, masks []uint8) {
	region := 1
	if d.registers[0x13]&1 != 0 {
		region = 0
	}
	for sy := range 8 {
		y := destY + sy
		if y < 0 || y >= Height {
			continue
		}
		py := sy
		if flipY {
			py = 7 - sy
		}
		for sx := range 8 {
			x := destX + sx
			if x < 0 || x >= Width {
				continue
			}
			px := sx
			if flipX {
				px = 7 - sx
			}
			pixel := d.tilePixel(region, tile, px, py)
			if pixel == 0 {
				continue
			}
			offset := y*Width + x
			if maskMode > 1 {
				masks[offset] = 1
				continue
			}
			if maskMode == 1 && masks[offset] == 0 {
				continue
			}
			if region == 0 {
				sprites[offset] = uint16(pixel)
			} else {
				sprites[offset] = uint16(palette*16) + uint16(pixel)
			}
			priorities[offset] = priorities[offset]&0xf0 | uint8(priority)
		}
	}
}

func (d *Device) drawSprites(sprites []uint16, priorities, masks []uint8) {
	region := uint32(1)
	if d.registers[0x13]&1 != 0 {
		region = 0
	}
	bankSize := uint32(0x100) << region
	start := uint32(d.registers[0x10]) << 1
	end := start + (uint32(d.registers[0x11])+1)*4
	for index := start; index+3 < end && index+3 < 0x10000; index += 4 {
		w0, w1 := d.vramWord(index), d.vramWord(index+1)
		w2, w3 := d.vramWord(index+2), d.vramWord(index+3)
		if w0&0x4000 == 0 || w3 == 0 {
			continue
		}
		x, y := int(w2&0x01ff), int(w0&0x01ff)
		if x >= 0x180 {
			x -= 0x200
		}
		if y >= 0x180 {
			y -= 0x200
		}
		bank, mask := uint32(w1>>12), int(w1>>8&3)
		flipX, flipY := w1&0x0800 != 0, w1&0x0400 != 0
		priority := int(w2 >> 9 & 3)
		xSize, ySize := 1<<int(w1&7), spriteYSize[w0>>9&0x0f]
		if w3&0x8000 != 0 || xSize == 1 && ySize == 1 {
			tile := int(bank*bankSize) + int(w3&0x03ff)
			d.drawSpriteTile(tile, int(w3>>12), flipX != (w3&0x0800 != 0), flipY != (w3&0x0400 != 0), x, y, priority, mask, sprites, priorities, masks)
			continue
		}
		for tileY := range ySize {
			for tileX := range xSize {
				data := d.vramWord((uint32(w3) << 1) + uint32(tileY*xSize+tileX))
				if data == 0 {
					continue
				}
				destX, destY := x+tileX*8, y+tileY*8
				if flipX {
					destX = x - (tileX+1)*8 + xSize*8
				}
				if flipY {
					destY = y - (tileY+1)*8 + ySize*8
				}
				destX &= 0x1ff
				if destX >= 0x180 {
					destX -= 0x200
				}
				tile := int(bank*bankSize) + int(data&0x03ff)
				d.drawSpriteTile(tile, int(data>>12), flipX != (data&0x0800 != 0), flipY != (data&0x0400 != 0), destX, destY, priority, mask, sprites, priorities, masks)
			}
		}
	}
}

func (d *Device) readSwapped(offset uint32) uint8 {
	offset &= VRAMSize - 1
	original := (offset &^ 0x7f) | ((offset >> 4) & 7) | ((offset & 0x0f) << 3)
	return d.ReadVRAM8(original)
}

// rozBitmapPixel reads the ROZ layer as a linear bitmap. Bcan takes this path
// when $F001F0 pixel mode is exactly $08 and the ROZ layer is in the 8bpp
// region; it skips the tilemap and tile graphics entirely and indexes VRAM by
// pixel. Base address, mask and palette bank all follow the decompiled
// renderer (see the knowledge base, docs/f003-video-mode.md section 7.3).
func (d *Device) rozBitmapPixel(x, y uint32) uint16 {
	xs, _ := tilemapDimensions(d.registers[0xc0])
	bit := 8 * (x + uint32(xs*8)*y)
	address := ((bit >> 3) + 4*uint32(d.registers[0xcb])) & (VRAMSize - 1)
	return uint16(d.ReadVRAM8(address)) + uint16(d.registers[0xc1]&0x0f)<<8
}

func (d *Device) rozPixel(x, y uint32) uint16 {
	mode := d.registers[0xc0]
	region := [4]int{4, 2, 1, 0}[mode&3]
	if region == 0 && d.pixelMode&0x18 == 0x08 {
		return d.rozBitmapPixel(x, y)
	}
	if region == 4 {
		count := (y>>3)*32 + (x >> 3)
		tile := 0x880 + int(count&7)*2
		if count&0x20 != 0 {
			tile ^= 1
		}
		tile |= int(count&0xc0) >> 2
		value := d.readSwapped(uint32(tile*8) + (y & 7))
		return uint16(value >> uint(7-(x&7)) & 1)
	}
	// ROZ 的 mode bit 1/0 是 region 選擇，不是 tilemap 那種全層 X/Y flip：
	// Bcan 的 ROZ 迴圈只用 `& 3`（region）、`& 0x20`（wrap）、`& 0xF00`（尺寸）
	// 與 `& 0x40`，沒有任何整層翻轉。
	xs, _ := tilemapDimensions(mode)
	entry := d.vramWord((uint32(d.registers[0xca]) << 1) + uint32((int(y)>>3)*xs+(int(x)>>3)))
	palette := int(entry >> 12)
	if d.registers[0xc1]&0x0200 != 0 {
		palette |= 8
	}
	tile := int(entry&0x03ff) + (int(d.registers[0xcb]&0xf000) >> 3)
	px, py := int(x&7), int(y&7)
	if entry&0x0800 != 0 {
		px ^= 7
	}
	if entry&0x0400 != 0 {
		py ^= 7
	}
	pixel := d.tilePixel(region, tile, px, py)
	if region == 0 {
		return uint16(pixel)
	}
	return uint16(palette*16) + uint16(pixel)
}

func (d *Device) drawROZ(priority int, indexed []uint16, priorities []uint8) {
	mode := d.registers[0xc0]
	xs, ys := tilemapDimensions(mode)
	region := [4]int{4, 2, 1, 0}[mode&3]
	transparent := uint16(1)
	if region == 0 {
		transparent = 0xff
	} else if region == 1 {
		transparent = 0x0f
	} else if region == 2 {
		transparent = 3
	}
	wrap := mode&0x20 != 0
	scrollX := uint32(d.registers[0xc2])<<16 | uint32(d.registers[0xc3])
	scrollY := uint32(d.registers[0xc4])<<16 | uint32(d.registers[0xc5])
	coefficientA, coefficientB := int32(int16(d.registers[0xc6])), int32(int16(d.registers[0xc7]))
	coefficientC, coefficientD := int32(int16(d.registers[0xc8])), int32(int16(d.registers[0xc9]))
	for y := range Height {
		lineA, lineScrollX, lineScrollY, enabled := d.rozLineParameters(y, coefficientA, scrollX, scrollY)
		if !enabled {
			continue
		}
		cx := int32(lineScrollX) + int32(y)*coefficientB
		cy := int32(lineScrollY) + int32(y)*coefficientD
		for x := range Width {
			sourceX, sourceY := cx>>8, cy>>8
			cx += lineA
			cy += coefficientC
			if !wrap && (sourceX < 0 || sourceX >= int32(xs*8) || sourceY < 0 || sourceY >= int32(ys*8)) {
				continue
			}
			pixel := d.rozPixel(uint32(sourceX)&uint32(xs*8-1), uint32(sourceY)&uint32(ys*8-1))
			offset := y*Width + x
			if pixel&transparent != 0 && priority < int(priorities[offset]>>4) {
				indexed[offset] = pixel
				priorities[offset] = priorities[offset]&0x0f | uint8(priority<<4)
			}
		}
	}
}

func (d *Device) rozLineParameters(y int, coefficientA int32, scrollX, scrollY uint32) (int32, uint32, uint32, bool) {
	mode := d.registers[0xc0]
	if mode&0x0200 != 0 || mode&0xf000 == 0 {
		return coefficientA, scrollX, scrollY, true
	}
	table0 := (uint32(d.registers[0xcc]) << 1) + uint32(y)
	deltaA := d.vramWord(table0)
	if deltaA == 0 {
		return 0, 0, 0, false
	}
	tableX := (uint32(d.registers[0xcd]) << 1) + uint32(y*2)
	tableY := (uint32(d.registers[0xcf]) << 1) + uint32(y*2)
	deltaX := uint32(d.vramWord(tableX))<<16 | uint32(d.vramWord(tableX+1))
	deltaY := uint32(d.vramWord(tableY))<<16 | uint32(d.vramWord(tableY+1))
	lineA := int32(int16(uint16(coefficientA) + deltaA))
	return lineA, scrollX + deltaX, scrollY + deltaY, true
}

func (d *Device) drawWindow(window, priority int, indexed []uint16, priorities []uint8) {
	baseIndex := 0xe8 + window*4
	control := d.registers[baseIndex]
	layerPriority := int(control >> 13 & 3)
	if priority != layerPriority {
		return
	}
	scrollX := int(d.registers[baseIndex+2] & 0x03ff)
	if scrollX&0x0200 != 0 {
		scrollX -= 0x0400
	}
	pen := uint16(control & 0xff)
	reverse := control&0x0800 != 0
	for y := range Height {
		yBase := 0
		if control&0x0100 != 0 {
			yBase = y * 2
		}
		base := (uint32(d.registers[baseIndex+1]) << 1) + uint32(yBase)
		minimum := int(int16(d.vramWord(base))) + scrollX
		maximum := int(int16(d.vramWord(base+1))) + scrollX
		for x := range Width {
			offset := y*Width + x
			if layerPriority >= int(priorities[offset]>>4) {
				continue
			}
			if (x >= minimum && x < maximum) != reverse {
				indexed[offset] = pen
				priorities[offset] = priorities[offset]&0x0f | uint8(layerPriority<<4)
			}
		}
	}
}

// RenderFrame derives RGB pixels from the current device state. It does not
// advance time and may therefore be called by headless tests or a frontend.
func (d *Device) RenderFrame() {
	d.RenderFrameLayers(AllLayers)
}

// RenderFrameLayers is a diagnostic renderer. A mask isolates chip layers
// without changing registers, VRAM, timing, or the normal all-layer path.
func (d *Device) RenderFrameLayers(layerMask uint8) {
	indexed := make([]uint16, Width*Height)
	priorities := make([]uint8, Width*Height)
	sprites := make([]uint16, Width*Height)
	masks := make([]uint8, Width*Height)
	for i := range priorities {
		priorities[i] = 0xff
	}
	if layerMask&LayerSprites != 0 {
		d.drawSprites(sprites, priorities, masks)
	}
	for priority := 7; priority >= 0; priority-- {
		for layer := range 3 {
			if layerMask&(LayerTilemap0<<layer) != 0 && d.videoFlags&(0x80>>layer) != 0 && int(d.registers[0x80+layer*0x10]>>13&7) == priority {
				d.drawTilemap(layer, priority, indexed, priorities)
			}
		}
		if layerMask&LayerROZ != 0 && d.videoFlags&4 != 0 && int(d.registers[0xc0]>>13&7) == priority {
			d.drawROZ(priority, indexed, priorities)
		}
		if layerMask&LayerWindows != 0 && d.videoFlags&2 != 0 {
			d.drawWindow(0, priority, indexed, priorities)
		}
		if layerMask&LayerWindows != 0 && d.videoFlags&2 != 0 && d.registers[0xec] != 0 {
			d.drawWindow(1, priority, indexed, priorities)
		}
	}
	if layerMask&LayerSprites != 0 && d.videoFlags&8 != 0 {
		for i, pixel := range sprites {
			if pixel != 0 && priorities[i]&0x0f <= priorities[i]>>4 {
				indexed[i] = pixel
			}
		}
	}
	width := 256
	if d.videoFlags&0x0100 != 0 {
		width = 320
	}
	for y := range Height {
		for x := range Width {
			color := uint32(0xff000000)
			if x < width {
				entry := d.palette[indexed[y*Width+x]&0xff]
				r := expand5(uint32(entry & 0x1f))
				g := expand5(uint32(entry >> 5 & 0x1f))
				b := expand5(uint32(entry >> 10 & 0x1f))
				color |= r<<16 | g<<8 | b
			}
			d.framebuffer[y*Width+x] = color
		}
	}
}

// expand5 把調色盤的 5 位元分量展開成 8 位元，複製高 3 位到低位，
// 使 $1F 對到 $FF 而不是 $F8。證據：Bcan 0.0.8b 的 320x240 截圖對同一 palette
// 值輸出 $21/$10/$73（confirmed-Bcan），與 MAME supracan driver 宣告的
// palette_device::xBGR_555（pal5bit）一致；兩個 oracle 同時支持此展開。
func expand5(value uint32) uint32 { return value<<3 | value>>2 }

func (d *Device) Framebuffer() []uint32 { return d.framebuffer[:] }

func (d *Device) FramebufferSHA256() [32]byte {
	buffer := make([]byte, len(d.framebuffer)*4)
	for index, pixel := range d.framebuffer {
		binary.BigEndian.PutUint32(buffer[index*4:], pixel)
	}
	return sha256.Sum256(buffer)
}

func (d *Device) NonblackPixels() uint32 {
	var count uint32
	for _, pixel := range d.framebuffer {
		if pixel&0x00ff_ffff != 0 {
			count++
		}
	}
	return count
}
