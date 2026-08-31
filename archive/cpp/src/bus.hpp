// Deprecated C++ oracle；production 改由純 Go 獨立實作。
// 68000 位址空間匯流排
// 規格出處：知識庫 docs/memory-map.md §2（全表 (a) 級確認）
//
//   $000000-$3FFFFF  卡帶 ROM 低區視圖（$000000-$000FFF 開機時 overlay IPL 4KB）
//   $E80000-$E8FFFF  音效共享 RAM 64KB（16-bit 存取，不做 byte 對調）
//   $E90000-$E9001F  UM6619 主機端埠（stub；$E9001C 為 sound CPU/lockout 控制）
//   $E90020-$E9003F  DMA 通道 0/1（stub）
//   $E90B3C-$E90B3D  lockout 檢查雜訊區（NOP 性質，此處給一個真實 word 儲存）
//   $EB0D00-$EB0D03  UMC6650（8-bit）
//   $EC0000-$ECFFFF  卡帶 SRAM（8-bit 寬，僅奇位址有效）
//   $F00000-$F001FF  UM6618 視訊暫存器（video/um6618.cpp）
//   $F00200-$F003FF  調色盤 RAM 256 色 xBGR-555（UM6618）
//   $F00400-$F3FFFF  no-op 區段
//   $F40000-$F5FFFF  VRAM 128KB（UM6618）
//   $F60000-$F7FFFF  no-op 區段
//   $F80000-$FBFFFF  卡帶 ROM 高區視圖（$F80000-$F80FFF 開機時 overlay IPL）
//   $FC0000-$FFFFFF  Work RAM 64KB，addr & 0xFFFF 映射（$FC-$FF 四頁同體）
//   越界 ROM 讀取回 0xFFFF，不丟 BusError
//
// overlay 控制（docs/bios-68k.md §2，IPL $604 處）：
//   $E9001C bit1 = 關閉低區（$000000）IPL overlay
//   $E9001C bit3 = 關閉高區（$F80000）IPL overlay
#pragma once

#include "umc6650.hpp"
#include "video/um6618.hpp"
#include "state.hpp"

#include <array>
#include <cstdint>
#include <functional>
#include <vector>

class SystemBus {
public:
    // 載入（loader 已做過 word-swap 還原）
    void loadRom(std::vector<uint8_t> rom);              // 卡帶映像（已還原）
    void loadIpl(const uint8_t *data, size_t len);       // internal_68k.bin（已還原）
    void loadSoundRam(const uint8_t *data, size_t len, uint32_t dst); // 6502 取樣資料
    void loadSram(const uint8_t *data, size_t len);      // 卡帶 SRAM（可選）

    // save state
    void saveState(StateWriter &w) const;
    void loadState(StateReader &r);
    uint64_t romHash() const;  // FNV-1a 64-bit（識別用，非加密）

    UMC6650 &lockout() { return lockout_; }
    UM6618 &video() { return video_; }
    const UM6618 &video() const { return video_; }

    // 手把狀態（16-bit active low，知識庫 memory-map.md §7 (b)）；
    // 65C02 未執行時經 MAME「direct mode」路徑由 $E80200/$E80202 讀出
    void setPad(int player, uint16_t bits) { pad_[player & 1] = bits; }
    uint16_t pad(int player) const { return pad_[player & 1]; }

    // 68k 寫 $E9000A/B → 觸發 65C02 IRQ bit5（sound-driver.md §4.1 (a)）
    std::function<void()> onSoundIrqRequest;

    // 68k 寫 $E80400-$E804FF（65C02 I/O 頁窗口）→ 轉發給 SoundCpu
    // （MAME _68k_soundram_w 行為；目前用於 latch $0404/$0405）
    std::function<void(uint16_t addr, uint8_t val)> onSoundIoWrite;

    // 供 65C02 wrapper 直接映射共享音效 RAM（$E80000 區與 65C02 空間同體，
    // docs/memory-map.md §5）
    uint8_t *soundRamData() { return soundRam_.data(); }

    // $E9001C 寫入時的通知（runner 用來 log overlay 狀態變化）
    std::function<void(uint16_t oldVal, uint16_t newVal)> onControlWrite;

    // FRC（$E90014/$16；行為依 MAME update_frc_state (b)，其本身即
    // case-by-case HACK——真實計時公式待查證）
    uint16_t frcControl() const { return frcControl_; }
    uint16_t frcFreq() const { return frcFreq_; }
    std::function<void()> onFrcWrite;

    uint8_t  read8(uint32_t addr) const;
    uint16_t read16(uint32_t addr) const;
    uint32_t read32(uint32_t addr) const {
        return (uint32_t(read16(addr)) << 16) | read16(addr + 2);
    }

    void write8(uint32_t addr, uint8_t val);
    void write16(uint32_t addr, uint16_t val);
    void write32(uint32_t addr, uint32_t val) {
        write16(addr, uint16_t(val >> 16));
        write16(addr + 2, uint16_t(val));
    }

    bool loOverlayOn() const { return !loOverlayOff_; }
    bool hiOverlayOn() const { return !hiOverlayOff_; }

    // 卡帶向量表（overlay 關閉後的 $0/$4，即 ROM 的 SSP/PC）
    uint32_t cartVector(int n) const {
        uint32_t off = uint32_t(n) * 4;
        if (off + 4 > rom_.size()) return 0xFFFFFFFF;
        return (uint32_t(rom_[off]) << 24) | (uint32_t(rom_[off + 1]) << 16) |
               (uint32_t(rom_[off + 2]) << 8) | rom_[off + 3];
    }

private:
    uint8_t romByte(uint32_t off) const;  // 越界回 0xFF

    std::vector<uint8_t> rom_;
    std::array<uint8_t, 4096>   ipl_{};
    std::array<uint8_t, 65536>  soundRam_{};
    std::array<uint8_t, 65536>  wram_{};
    std::array<uint8_t, 32768>  sram_{};     // 固定 32768 byte（memory-map.md §2 (a)）
    uint16_t e90b3c_ = 0;                     // 雜訊區（給真實儲存，行為無害）
    uint16_t ctrl_ = 0;                       // $E9001C
    // IPL overlay 關閉為單向 latch：遊戲上傳音效驅動時會把整個 $E9001C 清 0
    // （sound-driver.md §1.1），若 overlay 隨 bit 清除而恢復，卡帶自己的中斷
    // 向量會被 IPL 蓋回（全部指向 IPL 的 rte），遊戲無法運作。
    bool loOverlayOff_ = false, hiOverlayOff_ = false;
    uint16_t pad_[2] = { 0xFFFF, 0xFFFF };    // active low，無輸入 = 全 1
    uint16_t frcControl_ = 0, frcFreq_ = 0;   // $E90014/$16（FRC stub，TODO：IRQ3 計時器）
    UMC6650 lockout_{};
    UM6618 video_{};

    // 主機 DMA 通道 0/1（$E90020-$3F，memory-map.md §4 (b)）
    struct DmaChannel { uint32_t src = 0, dst = 0; uint16_t count = 0, control = 0; };
    std::array<DmaChannel, 2> dma_{};
    uint16_t dmaReg(int ch, int idx) const;
    void dmaSetReg(int ch, int idx, uint16_t v);
    void dmaWriteByte(int ch, int idx, uint32_t addr, uint8_t val);
    void dmaTrigger(int ch, uint16_t control);
};
