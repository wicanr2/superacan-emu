// UM6618 繪圖晶片（背景/動畫處理器）
// 規格出處：知識庫 docs/memory-map.md §3（(b)，MAME src/mame/umc/supracan.cpp），
// 渲染演算法依 MAME driver（BSD-3-Clause，(c) 2026 Angelo Salese / Ryan Holtz 等）
// 的行為描述重新實作，未複製其程式碼。
//
//   暫存器窗口 $F00000-$F001FF（256 個 16-bit）
//   調色盤 $F00200-$F003FF（256 色 xBGR-555）
//   VRAM   $F40000-$F5FFFF（128 KB，以 word 視圖操作，同 MAME m_vram[]）
//
// 已實作：3 個 tilemap 層（8/4/2 bpp、5 種尺寸、scroll、linescroll、
//         lineselect、mosaic、全層 flip、優先度混色）、sprite（含 mask
//         模式）、window 0、vblank/raster IRQ 旗標。
// TODO：ROZ 層（$180 起，MAME 本身也是部分實作）、window 1、1bpp 層、
//       逐行 partial update（目前為整幀渲染，mid-frame 改 scroll 會不準）。
#pragma once

#include <array>
#include <cstdint>
#include <functional>
#include <vector>

class UM6618 {
public:
    static constexpr int WIDTH = 320;   // 最大 X 寬（video flags bit8 決定 320/256）
    static constexpr int HEIGHT = 240;

    // 暫存器讀寫（offset = ($F00000 起 byte offset) >> 1）
    uint16_t readReg(uint16_t index);
    void writeReg(uint16_t index, uint16_t data);

    // 調色盤（xBGR-555）
    uint16_t readPalette(uint16_t index) const { return palette_[index & 0xFF]; }
    void writePalette(uint16_t index, uint16_t data) { palette_[index & 0xFF] = data; }

    // VRAM（byte 位址 0-$1FFFF）
    uint8_t readVramByte(uint32_t off) const;
    void writeVramByte(uint32_t off, uint8_t val);
    uint16_t readVramWord(uint32_t off) const {
        return uint16_t((readVramByte(off) << 8) | readVramByte(off + 1));
    }

    // 目前掃描線（由 runner 每線更新；262 線/幀）
    void setScanline(int vpos) { vpos_ = vpos; }
    int scanline() const { return vpos_; }
    uint64_t frameNumber() const { return frame_; }

    // 除錯用：直接讀原始暫存器/VRAM
    uint16_t rawReg(int i) const { return regs_[i & 0xFF]; }
    uint16_t vramWordRaw(uint32_t idx) const { return vram_[idx & 0xFFFF]; }
    uint16_t paletteRaw(int i) const { return palette_[i & 0xFF]; }

    // 水平模式：video flags bit8（true=320 寬）
    bool h320() const { return videoFlags_ & 0x100; }

    // IRQ 狀態（runner 用）：vblank 在 vpos==240 觸發（irq mask bit7，mask 在
    // $E90010）；raster 在每條可視線觸發（mask bit4）。$F00000 讀取會清
    // vblank 旗標（MAME video_r case 0 的行為）。
    bool vblankPending() const { return vblankIrq_; }
    void triggerVblank();
    void clearVblankPulse() { vblankIrq_ = false; }
    bool rasterActive() const { return rasterIrq_; }
    void triggerRaster() { rasterIrq_ = true; }
    void clearRaster() { rasterIrq_ = false; }

    // IRQ5 line on/off（$F0000A/$F0000C 寫 bit15 + 目標線；MAME line_on/off_cb）
    // 到達目標線觸發 IRQ5（脈衝）；line off 目標線解除。
    void tickLineIrq() {  // runner 在每條線推進時呼叫（vpos 已更新）
        if (lineOn_ >= 0 && vpos_ == lineOn_) irq5_ = true;
        if (lineOff_ >= 0 && vpos_ == lineOff_) irq5_ = false;
    }
    bool irq5Pending() const { return irq5_; }
    void clearIrq5() { irq5_ = false; }

    // 渲染整幀到 framebuffer_（RGBX8888，WIDTH*HEIGHT）
    void renderFrame();
    const std::vector<uint32_t> &framebuffer() const { return framebuffer_; }

    // sprite DMA 需要回寫 SystemBus（MAME 以 address_space 讀寫）
    std::function<uint16_t(uint32_t)> busRead16;
    std::function<void(uint32_t, uint16_t)> busWrite16;

    uint8_t irqMask() const { return irqMask_; }
    void setIrqMask(uint8_t v) { irqMask_ = v; }

private:
    // ---- 暫存器衍生狀態（對齊 MAME 欄位） ----
    std::array<uint16_t, 256> regs_{};
    std::array<uint16_t, 0x10000> vram_{};      // word 視圖，128 KB
    std::array<uint16_t, 256> palette_{};

    uint16_t videoFlags_ = 0;
    uint32_t tilemapBase_[3]{};
    int tilemapScrollX_[3]{};
    int tilemapScrollY_[3]{};
    uint16_t tilemapFlags_[3]{};
    uint16_t tilemapMode_[3]{};
    uint16_t tilemapTileMode_[3]{};
    uint16_t tilemapLinescrollAddr_[3]{};
    uint16_t tilemapLineselectAddr_[3]{};

    uint32_t spriteBaseAddr_ = 0;
    uint32_t spriteCount_ = 0;
    uint16_t spriteFlags_ = 0;

    uint16_t windowControl_[2]{};
    uint16_t windowStartAddr_[2]{};
    uint16_t windowScrollX_[2]{};
    uint16_t windowScrollY_[2]{};

    // sprite DMA
    uint32_t sprDmaSrc_ = 0, sprDmaDst_ = 0;
    uint16_t sprDmaSrcInc_ = 0, sprDmaDstInc_ = 0, sprDmaCount_ = 0;

    uint16_t frcReg_ = 0;   // $1F0：pixel mode / gfx mode（讀回用）
    uint8_t irqMask_ = 0;

    int vpos_ = 0;
    uint64_t frame_ = 0;
    bool vblankIrq_ = false;
    bool rasterIrq_ = false;
    int lineOn_ = -1, lineOff_ = -1;  // IRQ5 line on/off 目標掃描線
    bool irq5_ = false;

    std::vector<uint32_t> framebuffer_;                 // RGBX8888 輸出
    std::array<uint16_t, WIDTH * HEIGHT> indexed_{};    // 調色盤索引
    std::array<uint8_t, WIDTH * HEIGHT> prio_{};        // 高 nibble=tilemap 優先、低 nibble=sprite
    std::array<uint16_t, WIDTH * HEIGHT> spriteBuf_{};
    std::array<uint8_t, WIDTH * HEIGHT> maskBuf_{};

    // ---- 內部 ----
    int tilemapRegion(int layer) const;                 // 0=8bpp 1=4bpp 2=2bpp
    void tilemapDimensions(int &xs, int &ys, int layer) const;
    int fetchTilePixel(int region, int tile, int x, int y) const;
    uint16_t tilemapPixel(int layer, int x, int y) const;  // 回調色盤索引（0=透明）
    void drawTilemapLayer(int layer, int priority);
    void drawSprites();
    void drawSpriteTile(int tile, int palette, bool xf, bool yf,
                        int dstx, int dsty, int prio, int maskMode);
    void drawWindow(int priority);
};
