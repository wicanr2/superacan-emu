// UM6618 繪圖晶片 — 實作
// 渲染行為依 MAME src/mame/umc/supracan.cpp（BSD-3-Clause）重新實作；
// 暫存器布局見知識庫 docs/memory-map.md §3。
#include "um6618.hpp"

#include <algorithm>
#include <cstdio>
#include <cstdlib>
#include <cstring>

extern uint32_t g_dbgPc;  // bus.cpp 除錯用

namespace {
// tilemap 尺寸選擇（MAME get_tilemap_dimensions）：flags bit11-8
// 0x200→16x16, 0x400→32x32, 0x600→64x32, 0xa00→128x32, 0xc00→64x64，預設 32x32
// sprite Y 尺寸表（MAME draw_sprites ysizes_table，值即 8px tile 列數）
constexpr int kSpriteYSizes[16] = { 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 12, 16, 20, 22, 24, 26 };
} // namespace

uint8_t UM6618::readVramByte(uint32_t off) const {
    off &= 0x1FFFF;
    uint16_t w = vram_[off >> 1];
    return (off & 1) ? uint8_t(w) : uint8_t(w >> 8);
}

void UM6618::writeVramByte(uint32_t off, uint8_t val) {
    off &= 0x1FFFF;
    uint16_t &w = vram_[off >> 1];
    w = (off & 1) ? uint16_t((w & 0xFF00) | val) : uint16_t((w & 0x00FF) | (val << 8));
}

uint16_t UM6618::readReg(uint16_t index) {
    index &= 0xFF;
    switch (index) {
    case 0x00: {  // Video IRQ flags：bit15=vblank 中、bit1=奇數幀；讀取清 vblank IRQ
        uint16_t data = (vpos_ >= 240) ? 0x8000 : 0;
        if (frame_ & 1) data |= 2;
        vblankIrq_ = false;
        return data;
    }
    case 0x01: return uint16_t(vpos_);        // 目前掃描線
    case 0x04: return videoFlags_;            // $08：video flags 讀回（gambling lord 依賴）
    case 0xF8: return frcReg_;                // $1F0：pixel mode | gfx mode
    default:   return regs_[index];
    }
}

void UM6618::writeReg(uint16_t index, uint16_t data) {
    index &= 0xFF;
    regs_[index] = data;
    switch (index) {
    // sprite DMA（$F00010-$F0001E；MAME video_w case 0x10-0x1e）
    case 0x08: sprDmaCount_ = data; break;
    case 0x09: sprDmaDst_ = (sprDmaDst_ & 0x0000FFFF) | (uint32_t(data) << 16); break;
    case 0x0A: sprDmaDst_ = (sprDmaDst_ & 0xFFFF0000) | data; break;
    case 0x0B: sprDmaDstInc_ = data; break;
    case 0x0C: sprDmaSrc_ = (sprDmaSrc_ & 0x0000FFFF) | (uint32_t(data) << 16); break;
    case 0x0D: sprDmaSrc_ = (sprDmaSrc_ & 0xFFFF0000) | data; break;
    case 0x0E: sprDmaSrcInc_ = data; break;
    case 0x0F:
        if (data & 0x8000) {
            if (std::getenv("ACAN_DMA"))
                std::fprintf(stderr, "[sprdma] src=$%08X dst=$%08X count=$%04X ctrl=$%04X inc=$%04X/$%04X (pc=$%08X)\n",
                             sprDmaSrc_, sprDmaDst_, sprDmaCount_, data, sprDmaSrcInc_, sprDmaDstInc_, g_dbgPc);
            if ((data & 0x2000) || (data & 0x4000)) sprDmaDst_ |= 0xF40000;
            for (uint32_t i = 0; i <= sprDmaCount_ && busWrite16 && busRead16; i++) {
                if (data & 0x0100) {  // 0 填充模式
                    busWrite16(sprDmaDst_, 0);
                    sprDmaDst_ += 2 * sprDmaDstInc_;
                } else {
                    busWrite16(sprDmaDst_, busRead16(sprDmaSrc_));
                    sprDmaDst_ += 2 * sprDmaDstInc_;
                    sprDmaSrc_ += 2 * sprDmaSrcInc_;
                }
            }
        }
        break;

    case 0x04: videoFlags_ = data; break;     // $08 video flags
    case 0x05:                                // $0A：raster line on IRQ 觸發線
        lineOn_ = (data & 0x8000) ? (data & 0xFF) : -1; break;
    case 0x06:                                // $0C：raster line off IRQ 觸發線
        lineOff_ = (data & 0x8000) ? (data & 0xFF) : -1; break;

    // sprite 全域設定（$F00020-$26）
    case 0x10: spriteBaseAddr_ = uint32_t(data) << 2; break;
    case 0x11: spriteCount_ = uint32_t(data) + 1; break;
    case 0x13: spriteFlags_ = data; break;

    // tilemap 0/1/2（各 16 byte 參數區，base $100/$120/$140）
    case 0x80: case 0x90: case 0xA0: tilemapFlags_[(index - 0x80) >> 4] = data; break;
    case 0x81: case 0x91: case 0xA1: tilemapTileMode_[(index - 0x81) >> 4] = data; break;
    case 0x82: case 0x92: case 0xA2: tilemapScrollX_[(index - 0x82) >> 4] = data; break;
    case 0x83: case 0x93: case 0xA3: tilemapScrollY_[(index - 0x83) >> 4] = data; break;
    case 0x84: case 0x94: case 0xA4: tilemapBase_[(index - 0x84) >> 4] = uint32_t(data) << 1; break;
    case 0x85: case 0x95: case 0xA5: tilemapMode_[(index - 0x85) >> 4] = data; break;
    case 0x86: case 0x96: case 0xA6: tilemapLinescrollAddr_[(index - 0x86) >> 4] = data; break;
    case 0x87: case 0x97: case 0xA7: tilemapLineselectAddr_[(index - 0x87) >> 4] = data; break;

    // ROZ（$180-$19E）：暫存器保留（regs_），渲染 stub — TODO

    // window 0/1（$1D0-$1DE）
    case 0xE8: case 0xEC: windowControl_[(index - 0xE8) >> 2] = data; break;
    case 0xE9: case 0xED: windowStartAddr_[(index - 0xE9) >> 2] = data; break;
    case 0xEA: case 0xEE: windowScrollX_[(index - 0xEA) >> 2] = data; break;
    case 0xEB: case 0xEF: windowScrollY_[(index - 0xEB) >> 2] = data; break;

    case 0xF8: frcReg_ = data & 0x1F; break;  // $1F0：pixel mode (bit4-3) + gfx mode (bit2-0)
    default: break;
    }
}

void UM6618::triggerVblank() {
    frame_++;
    if (irqMask_ & 0x80) vblankIrq_ = true;
}

int UM6618::tilemapRegion(int layer) const {
    // MAME get_tilemap_region：layer0/1 依 gfx_mode（$1F0 低 3 bit）；layer2 固定 2bpp
    const int gfxMode = frcReg_ & 7;
    switch (layer) {
    case 0: { static const int m[8] = { 2, 1, 0, 1, 0, 0, 0, 0 }; return m[gfxMode]; }
    case 1: { static const int m[8] = { 2, 1, 1, 1, 2, 2, 2, 2 }; return m[gfxMode]; }
    default: return 2;
    }
}

void UM6618::tilemapDimensions(int &xs, int &ys, int layer) const {
    xs = ys = 32;
    switch (tilemapFlags_[layer] & 0x0F00) {
    case 0x200: xs = ys = 16; break;
    case 0x400: xs = ys = 32; break;
    case 0x600: xs = 64; ys = 32; break;
    case 0xA00: xs = 128; ys = 32; break;
    case 0xC00: xs = ys = 64; break;
    default: break;
    }
}

int UM6618::fetchTilePixel(int region, int tile, int x, int y) const {
    // tile 像素布局同 MAME gfx_layout（8bpp=64B、4bpp=32B、2bpp=16B 線性 packed）
    uint32_t off;
    switch (region) {
    case 0: off = uint32_t(tile) * 64 + y * 8 + x;
        return readVramByte(off);
    case 1: off = uint32_t(tile) * 32 + y * 4 + x / 2;
        return (x & 1) ? (readVramByte(off) >> 4) : (readVramByte(off) & 0xF);
    default: off = uint32_t(tile) * 16 + y * 2 + x / 4;
        return (readVramByte(off) >> ((x & 3) * 2)) & 3;
    }
}

uint16_t UM6618::tilemapPixel(int layer, int x, int y) const {
    // x/y 已 wrap 在 tilemap 像素座標內；回傳調色盤索引（像素值 0 = 透明）
    int xs, ys;
    tilemapDimensions(xs, ys, layer);
    const int region = tilemapRegion(layer);
    const uint32_t base = tilemapBase_[layer];
    const int gfxMode = (tilemapMode_[layer] & 0x7000) >> 12;

    const int tx = x >> 3, ty = y >> 3;
    const uint32_t count = (base + uint32_t(ty * xs + tx)) & 0xFFFF;
    const uint16_t entry = vram_[count];

    uint8_t palBase = (entry & 0xF000) >> 12;
    if (tilemapTileMode_[layer] & 0x0200) palBase |= 8;  // MAME: tile_mode bit9

    const uint16_t tileBank = uint16_t(gfxMode << (8 + region));
    const int tile = (entry & 0x03FF) + tileBank;
    int px = x & 7, py = y & 7;
    if (entry & 0x0800) px ^= 7;   // tile 內 X flip
    if (entry & 0x0400) py ^= 7;   // tile 內 Y flip

    const int pix = fetchTilePixel(region, tile, px, py);
    if (region == 0) return uint16_t(pix);
    // 4bpp/2bpp 皆為 palBase*16 + pix（MAME palette_shift/granularity 等效）
    return uint16_t(palBase * 16 + pix);
}

void UM6618::drawTilemapLayer(int layer, int priority) {
    int xs, ys;
    tilemapDimensions(xs, ys, layer);
    const int region = tilemapRegion(layer);
    const uint16_t transmask = (region == 0) ? 0xFF : (region == 1) ? 0x0F : 0x03;
    const bool wrap = tilemapFlags_[layer] & 0x20;

    int scrollx = tilemapScrollX_[layer] & 0xFFF;
    int scrolly = tilemapScrollY_[layer] & 0xFFF;
    if (scrollx & 0x800) scrollx -= 0x1000;
    if (scrolly & 0x800) scrolly -= 0x1000;
    // 全層 flip 會反轉 scroll 意義（MAME：formduel / sangofgt）
    if (tilemapFlags_[layer] & 2) scrollx ^= (xs * 8) - 1;
    if (tilemapFlags_[layer] & 1) scrolly ^= (ys * 8) - 1;

    const int mosaicCount = (tilemapFlags_[layer] & 0x001C) >> 2;
    const int mosaicMask = int(0xFFFFFFFFu << mosaicCount);

    for (int y = 0; y < HEIGHT; y++) {
        const int actualy = y & mosaicMask;
        int realy = actualy + scrolly;
        if (!wrap && (scrolly + y < 0 || scrolly + y > ys * 8 - 1)) continue;

        // lineselect（tile_mode bit11）：逐行改 Y 來源
        if (tilemapTileMode_[layer] & 0x0800) {
            const int16_t ls = int16_t(vram_[((uint32_t(tilemapLineselectAddr_[layer]) << 1) + y) & 0xFFFF]);
            realy = (ls + scrolly) & (ys * 8 - 1);
        }
        realy &= ys * 8 - 1;

        int lineScrollX = scrollx;
        // linescroll（tile_mode bit14）：逐行改 X scroll
        if (tilemapTileMode_[layer] & 0x4000) {
            const int16_t lsx = int16_t(vram_[((uint32_t(tilemapLinescrollAddr_[layer]) << 1) + y) & 0xFFFF]);
            lineScrollX += lsx;
        }

        uint16_t *dst = &indexed_[y * WIDTH];
        uint8_t *priop = &prio_[y * WIDTH];
        for (int x = 0; x < WIDTH; x++) {
            const int actualx = x & mosaicMask;
            const int realx = actualx + lineScrollX;
            if (!wrap && (lineScrollX + x < 0 || lineScrollX + x > xs * 8 - 1)) continue;

            const uint16_t srcpix = tilemapPixel(layer, realx & (xs * 8 - 1), realy);
            if ((srcpix & transmask) != 0 && priority < (priop[x] >> 4)) {
                dst[x] = srcpix;
                priop[x] = uint8_t((priop[x] & 0x0F) | (priority << 4));
            }
        }
    }
}

void UM6618::drawSpriteTile(int tile, int palette, bool xf, bool yf,
                            int dstx, int dsty, int prio, int maskMode) {
    const int region = (spriteFlags_ & 1) ? 0 : 1;  // 8bpp : 4bpp
    for (int sy = 0; sy < 8; sy++) {
        const int y = dsty + sy;
        if (y < 0 || y >= HEIGHT) continue;
        const int py = yf ? 7 - sy : sy;
        for (int sx = 0; sx < 8; sx++) {
            const int x = dstx + sx;
            if (x < 0 || x >= WIDTH) continue;
            const int px = xf ? 7 - sx : sx;
            const int pix = fetchTilePixel(region, tile, px, py);
            if (!pix) continue;
            const size_t o = size_t(y) * WIDTH + x;
            if (maskMode > 1) { maskBuf_[o] = 1; continue; }        // 只建 mask
            if (maskMode == 1 && maskBuf_[o] == 0) continue;        // masked：只在 mask 處畫
            spriteBuf_[o] = uint16_t(region == 0 ? pix : palette * 16 + pix);
            prio_[o] = (prio_[o] & 0xF0) | uint8_t(prio);
        }
    }
}

void UM6618::drawSprites() {
    // MAME draw_sprites：sprite 表在 VRAM，每筆 4 word
    const int region = (spriteFlags_ & 1) ? 0 : 1;
    const uint32_t bankSize = 0x100u << region;
    const uint32_t startWord = spriteBaseAddr_ >> 1;
    const uint32_t endWord = startWord + spriteCount_ * 4;

    for (uint32_t i = startWord; i + 3 < endWord && i + 3 < vram_.size(); i += 4) {
        const uint16_t w0 = vram_[i], w1 = vram_[i + 1], w2 = vram_[i + 2], w3 = vram_[i + 3];
        if (!(w0 & 0x4000) || !w3) continue;   // enable bit

        int x = w2 & 0x01FF;
        int y = w0 & 0x01FF;
        if (y >= 0x180) y -= 0x200;            // 9-bit 環繞
        if (x >= 0x180) x -= 0x200;

        const uint32_t bank = (w1 & 0xF000) >> 12;
        const int mask = (w1 & 0x0300) >> 8;
        const bool sxf = w1 & 0x0800, syf = w1 & 0x0400;
        const int prio = (w2 >> 9) & 3;
        const uint16_t ptr = w3;
        const int xsize = 1 << (w1 & 7);
        const int ysize = kSpriteYSizes[(w0 & 0x1E00) >> 9];

        if ((ptr & 0x8000) || (xsize == 1 && ysize == 1)) {
            // direct sprite：tile 資訊直接來自 ptr
            const int tile = int(bank * bankSize) + (ptr & 0x03FF);
            const int palette = (ptr & 0xF000) >> 12;
            const bool txf = sxf != bool(ptr & 0x0800);
            const bool tyf = syf != bool(ptr & 0x0400);
            drawSpriteTile(tile, palette, txf, tyf, x, y, prio, mask);
        } else {
            // 查表：xsize×ysize 個子項目
            for (int yt = 0; yt < ysize; yt++) {
                for (int xt = 0; xt < xsize; xt++) {
                    const uint16_t data = vram_[((uint32_t(ptr) << 1) + yt * xsize + xt) & 0xFFFF];
                    if (!data) continue;
                    const int tile = int(bank * bankSize) + (data & 0x03FF);
                    const int palette = (data & 0xF000) >> 12;
                    int xpos = sxf ? (x - (xt + 1) * 8 + xsize * 8) : (x + xt * 8);
                    const int ypos = syf ? (y - (yt + 1) * 8 + ysize * 8) : (y + yt * 8);
                    xpos &= 0x1FF;
                    if (xpos >= 0x180) xpos -= 0x200;
                    const bool txf = sxf != bool(data & 0x0800);
                    const bool tyf = syf != bool(data & 0x0400);
                    drawSpriteTile(tile, palette, txf, tyf, xpos, ypos, prio, mask);
                }
            }
        }
    }
}

void UM6618::drawWindow(int priority) {
    // MAME window 0（video_flags bit1 觸發）；window 1 尚無遊戲使用（TODO）
    const int layerPrio = (windowControl_[0] >> 13) & 3;
    if (priority != layerPrio) return;
    const bool reverseClip = windowControl_[0] & 0x0800;
    int scrollx = windowScrollX_[0] & 0x3FF;
    if (scrollx & 0x200) scrollx -= 0x400;
    const uint8_t pen = windowControl_[0] & 0xFF;

    for (int y = 0; y < HEIGHT; y++) {
        const int ybase = (windowControl_[0] & 0x0100) ? y * 2 : 0;
        const uint32_t base = ((uint32_t(windowStartAddr_[0]) << 1) + ybase) & 0xFFFF;
        const int minx = int16_t(vram_[base]) + scrollx;
        const int maxx = int16_t(vram_[base + 1]) + scrollx;
        for (int x = 0; x < WIDTH; x++) {
            const size_t o = size_t(y) * WIDTH + x;
            if (layerPrio >= (prio_[o] >> 4)) continue;
            if ((x >= minx && x < maxx) != reverseClip) {
                indexed_[o] = pen;
                prio_[o] = uint8_t((prio_[o] & 0x0F) | (layerPrio << 4));
            }
        }
    }
}

void UM6618::renderFrame() {
    indexed_.fill(0);
    prio_.fill(0xFF);
    spriteBuf_.fill(0);
    maskBuf_.fill(0);

    drawSprites();

    // 優先度 7（最底）→ 0（最頂）；每層獨立優先度（flags bit15-13）
    static const int layerMask = [] {
        const char *e = std::getenv("ACAN_LAYERMASK");  // 除錯：bit0-2=tilemap0-2, bit3=sprite
        return e ? std::atoi(e) : 0xF;
    }();
    for (int pri = 7; pri >= 0; pri--) {
        for (int layer = 0; layer < 3; layer++) {
            if (!(layerMask & (1 << layer))) continue;
            if (!(videoFlags_ & (0x80 >> layer))) continue;   // bit7/6/5 = tilemap0/1/2
            if (int((tilemapFlags_[layer] >> 13) & 7) != pri) continue;
            drawTilemapLayer(layer, pri);
        }
        // ROZ 層（video flags bit2）：TODO（MAME 亦未完成）
        if (videoFlags_ & 0x2) drawWindow(pri);
    }

    // 合成 sprite：sprite 優先度 <= tilemap 優先度時蓋上
    if ((videoFlags_ & 0x8) && (layerMask & 0x8)) {
        for (size_t o = 0; o < indexed_.size(); o++) {
            if (spriteBuf_[o] == 0) continue;
            if ((prio_[o] & 0x0F) <= (prio_[o] >> 4)) indexed_[o] = spriteBuf_[o];
        }
    }

    // 調色盤轉 RGBX8888（xBGR-555：bit14-10=B、9-5=G、4-0=R）
    if (framebuffer_.size() != size_t(WIDTH) * HEIGHT)
        framebuffer_.resize(size_t(WIDTH) * HEIGHT);
    const int w = h320() ? 320 : 256;
    for (int y = 0; y < HEIGHT; y++) {
        for (int x = 0; x < WIDTH; x++) {
            uint32_t rgb = 0xFF000000;  // 256 模式右側補黑
            if (x < w) {
                const uint16_t c = palette_[indexed_[y * WIDTH + x] & 0xFF];
                const uint32_t r = (c & 0x1F) << 3, g = ((c >> 5) & 0x1F) << 3, b = ((c >> 10) & 0x1F) << 3;
                rgb = 0xFF000000 | (r << 16) | (g << 8) | b;
            }
            framebuffer_[y * WIDTH + x] = rgb;
        }
    }
}
