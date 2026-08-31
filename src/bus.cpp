#include "bus.hpp"

#include <algorithm>
#include <cstring>
#include <cstdio>
#include <cstdlib>

uint32_t g_dbgPc = 0;  // 除錯：main 每條指令前更新
uint64_t g_dbgFrame = 0;  // 除錯：main 每幀更新（ACAN_TRACE65 用）

void SystemBus::loadRom(std::vector<uint8_t> rom) { rom_ = std::move(rom); }

void SystemBus::loadIpl(const uint8_t *data, size_t len) {
    std::memcpy(ipl_.data(), data, std::min(len, ipl_.size()));
}

void SystemBus::loadSoundRam(const uint8_t *data, size_t len, uint32_t dst) {
    if (dst >= soundRam_.size()) return;
    std::memcpy(soundRam_.data() + dst, data, std::min(len, soundRam_.size() - dst));
}

void SystemBus::loadSram(const uint8_t *data, size_t len) {
    std::memcpy(sram_.data(), data, std::min(len, sram_.size()));
}

uint8_t SystemBus::romByte(uint32_t off) const {
    return off < rom_.size() ? rom_[off] : 0xFF;
}

uint8_t SystemBus::read8(uint32_t addr) const {
    addr &= 0xFFFFFF;
    if (addr < 0x400000) {
        if (addr < 0x1000 && loOverlayOn()) return ipl_[addr];
        return romByte(addr);
    }
    if (addr >= 0xE80000 && addr < 0xE90000) {
        // MAME direct mode：$E80200/$E80202 直接讀手把（active low ^0xFFFF，
        // 無輸入時回 0）；65C02 未執行時遊戲 polling 靠此路徑
        if (addr == 0xE80200 || addr == 0xE80201) {
            uint16_t v = pad_[0] ^ 0xFFFF;
            return (addr & 1) ? uint8_t(v) : uint8_t(v >> 8);
        }
        if (addr == 0xE80202 || addr == 0xE80203) {
            uint16_t v = pad_[1] ^ 0xFFFF;
            return (addr & 1) ? uint8_t(v) : uint8_t(v >> 8);
        }
        return soundRam_[addr & 0xFFFF];
    }
    if (addr >= 0xE90000 && addr < 0xE91000) {
        // $E9001C/$E9001D：sound CPU / lockout 控制暫存器（16-bit）
        if (addr == 0xE9001C) return uint8_t(ctrl_ >> 8);
        if (addr == 0xE9001D) return uint8_t(ctrl_);
        if (addr == 0xE90B3C) return uint8_t(e90b3c_ >> 8);
        if (addr == 0xE90B3D) return uint8_t(e90b3c_);
        // $E90010：68k IRQ mask（8-bit，bit7=vblank、bit4=raster，§6 (b)）
        if (addr == 0xE90010 || addr == 0xE90011) return video_.irqMask();
        // $E90014/$16：FRC control/frequency（stub 儲存；IRQ3 計時器 TODO）
        if (addr == 0xE90014) return uint8_t(frcControl_ >> 8);
        if (addr == 0xE90015) return uint8_t(frcControl_);
        if (addr == 0xE90016) return uint8_t(frcFreq_ >> 8);
        if (addr == 0xE90017) return uint8_t(frcFreq_);
        // $E90004/05：6502 取樣 DMA 位址回報；$E9000C/0D：6502 IRQ 旗標（MAME (b)）
        if (addr == 0xE90004 || addr == 0xE90005) return soundRam_[0x040C + (addr - 0xE90004)];
        if (addr == 0xE9000C || addr == 0xE9000D) return soundRam_[0x040A];
        return 0;  // UM6619 host port / DMA 讀回 stub：讀 0
    }
    if (addr >= 0xEB0D00 && addr <= 0xEB0D03) {
        // UMC6650（8-bit）：$EB0D01=資料埠（讀）、$EB0D03=位址埠（讀回目前位址）
        if (addr == 0xEB0D01) return lockout_.readDataPort();
        return 0xFF;
    }
    if (addr >= 0xEC0000 && addr < 0xED0000) {
        // SRAM 8-bit 寬，僅奇位址有效（memory-map.md §2 (a)）
        if (addr & 1) return sram_[(addr & 0xFFFF) >> 1];
        return 0xFF;
    }
    if (addr >= 0xF00000 && addr < 0xF00200) {
        uint16_t w = const_cast<UM6618 &>(video_).readReg((addr & 0x1FF) >> 1);
        return (addr & 1) ? uint8_t(w) : uint8_t(w >> 8);
    }
    if (addr >= 0xF00200 && addr < 0xF00400) {
        uint16_t w = video_.readPalette((addr & 0x1FF) >> 1);
        return (addr & 1) ? uint8_t(w) : uint8_t(w >> 8);
    }
    if (addr >= 0xF40000 && addr < 0xF60000) return video_.readVramByte(addr & 0x1FFFF);
    if (addr >= 0xF80000 && addr < 0xFC0000) {
        uint32_t off = addr & 0x3FFFF;
        if (off < 0x1000 && hiOverlayOn()) return ipl_[off];
        return romByte(off);
    }
    if (addr >= 0xFC0000) return wram_[addr & 0xFFFF];
    return 0xFF;  // 未映射 / no-op 區段
}

uint16_t SystemBus::read16(uint32_t addr) const {
    return (uint16_t(read8(addr)) << 8) | read8(addr + 1);
}

void SystemBus::write8(uint32_t addr, uint8_t val) {
    addr &= 0xFFFFFF;
    if (addr < 0x400000) return;  // ROM 區：忽略寫入
    if (addr >= 0xE80000 && addr < 0xE90000) {
        if (std::getenv("ACAN_WATCH") && addr >= 0xE80300 && addr < 0xE80310)
            std::fprintf(stderr, "[watch] $%08X <- $%02X (pc=$%08X)\n", addr, val, g_dbgPc);
        soundRam_[addr & 0xFFFF] = val;
        // 65C02 I/O 頁（$0400-$04FF）轉發（MAME _68k_soundram_w 行為）
        if ((addr & 0xFF00) == 0x0400 && onSoundIoWrite) onSoundIoWrite(uint16_t(addr & 0xFFFF), val);
        return;
    }
    if (addr >= 0xE90000 && addr < 0xE91000) {
        if (addr == 0xE90B3C) { e90b3c_ = uint16_t((e90b3c_ & 0x00FF) | (val << 8)); return; }
        if (addr == 0xE90B3D) { e90b3c_ = uint16_t((e90b3c_ & 0xFF00) | val); return; }
        if (addr == 0xE90010 || addr == 0xE90011) { video_.setIrqMask(val); return; }
        if (addr == 0xE90014) { frcControl_ = uint16_t((frcControl_ & 0x00FF) | (val << 8)); return; }
        if (addr == 0xE90015) { frcControl_ = uint16_t((frcControl_ & 0xFF00) | val); return; }
        if (addr == 0xE90016) { frcFreq_ = uint16_t((frcFreq_ & 0x00FF) | (val << 8)); return; }
        if (addr == 0xE90017) { frcFreq_ = uint16_t((frcFreq_ & 0xFF00) | val); return; }
        if (addr >= 0xE90020 && addr < 0xE90040) {
            // 主機 DMA ch0/ch1：16-bit 暫存器，byte 寫入做 read-modify-write
            const int ch = (addr >> 4) & 1;
            const uint16_t idx = (addr >> 1) & 7;
            // 目前只保留 control 觸發所需資訊；透過 write16 路徑處理
            dmaWriteByte(ch, idx, addr, val);
            return;
        }
        if (addr == 0xE9001C || addr == 0xE9001D) {
            uint16_t oldV = ctrl_;
            if (addr == 0xE9001C) ctrl_ = uint16_t((ctrl_ & 0x00FF) | (val << 8));
            else                  ctrl_ = uint16_t((ctrl_ & 0xFF00) | val);
            if (ctrl_ & 0x0002) loOverlayOff_ = true;   // 單向 latch
            if (ctrl_ & 0x0008) hiOverlayOff_ = true;
            if (ctrl_ != oldV && onControlWrite) onControlWrite(oldV, ctrl_);
            return;
        }
        if (addr == 0xE9000A || addr == 0xE9000B) {
            // 68k→65C02 命令通知（IRQ bit5）
            if (std::getenv("ACAN_TRACE65"))
                std::fprintf(stderr, "[68k] f=%llu $E9000A <- $%02X (pc=$%08X)\n",
                             (unsigned long long)g_dbgFrame, val, g_dbgPc);
            if (onSoundIrqRequest) onSoundIrqRequest();
            return;
        }
        return;  // UM6619 host port stub：寫入忽略
    }
    if (addr >= 0xEB0D00 && addr <= 0xEB0D03) {
        // UMC6650：$EB0D03=位址埠（寫）、$EB0D01=資料埠（寫）
        if (addr == 0xEB0D03) lockout_.writeAddrPort(val);
        else if (addr == 0xEB0D01) lockout_.writeDataPort(val);
        return;
    }
    if (addr >= 0xEC0000 && addr < 0xED0000) {
        if (addr & 1) sram_[(addr & 0xFFFF) >> 1] = val;
        return;
    }
    if (addr >= 0xF00000 && addr < 0xF00200) {
        // UM6618 暫存器：16-bit，byte 寫入 read-modify-write
        const uint16_t idx = (addr & 0x1FF) >> 1;
        uint16_t w = video_.readReg(idx);
        w = (addr & 1) ? uint16_t((w & 0xFF00) | val) : uint16_t((w & 0x00FF) | (val << 8));
        video_.writeReg(idx, w);
        return;
    }
    if (addr >= 0xF00200 && addr < 0xF00400) {
        const uint16_t idx = (addr & 0x1FF) >> 1;
        uint16_t w = video_.readPalette(idx);
        w = (addr & 1) ? uint16_t((w & 0xFF00) | val) : uint16_t((w & 0x00FF) | (val << 8));
        video_.writePalette(idx, w);
        return;
    }
    if (addr >= 0xF40000 && addr < 0xF60000) { video_.writeVramByte(addr & 0x1FFFF, val); return; }
    if (addr >= 0xFC0000) {
        if (std::getenv("ACAN_WATCH")) {
            const uint32_t o = addr & 0xFFFF;
            if (o == 0x0020 || o == 0x001A || o == 0x0078 || o == 0x0424 || o == 0x0000 || o == 0x0001)
                std::fprintf(stderr, "[watch] $FC%04X <- $%02X (pc=$%08X)\n", o, val, g_dbgPc);
            if (o >= 0x80F8 && o < 0x8110)
                std::fprintf(stderr, "[watch2] $FC%04X <- $%02X (pc=$%08X)\n", o, val, g_dbgPc);
        }
        wram_[addr & 0xFFFF] = val; return;
    }
    // 其餘（含 $F00400-$F3FFFF、$F60000-$F7FFFF no-op 區段）：忽略
}

void SystemBus::write16(uint32_t addr, uint16_t val) {
    write8(addr, uint8_t(val >> 8));
    write8(addr + 1, uint8_t(val));
}

// ---- 主機 DMA（$E90020-$3F；行為依 MAME dma_w，(b)）----
uint16_t SystemBus::dmaReg(int ch, int idx) const {
    const DmaChannel &d = dma_[ch & 1];
    switch (idx & 7) {
    case 0: return uint16_t(d.src >> 16);
    case 1: return uint16_t(d.src);
    case 2: return uint16_t(d.dst >> 16);
    case 3: return uint16_t(d.dst);
    case 4: return d.count;
    default: return 0;
    }
}

void SystemBus::dmaSetReg(int ch, int idx, uint16_t v) {
    DmaChannel &d = dma_[ch & 1];
    switch (idx & 7) {
    case 0: d.src = (d.src & 0x0000FFFF) | (uint32_t(v) << 16); break;
    case 1: d.src = (d.src & 0xFFFF0000) | v; break;
    case 2: d.dst = (d.dst & 0x0000FFFF) | (uint32_t(v) << 16); break;
    case 3: d.dst = (d.dst & 0xFFFF0000) | v; break;
    case 4: d.count = v; break;
    default: break;
    }
}

void SystemBus::dmaWriteByte(int ch, int idx, uint32_t addr, uint8_t val) {
    if (idx == 5) {
        // control：16-bit 寫入拆成高/低 byte，低 byte 寫入時視為完整並觸發
        DmaChannel &d = dma_[ch & 1];
        d.control = (addr & 1) ? uint16_t((d.control & 0xFF00) | val)
                               : uint16_t((d.control & 0x00FF) | (val << 8));
        if ((addr & 1) && (d.control & 0x8800)) dmaTrigger(ch, d.control);
        else if ((addr & 1) && std::getenv("ACAN_DMA"))
            std::fprintf(stderr, "[dma] ch%d control=$%04X 未觸發（bit15/bit11 皆 0）(pc=$%08X)\n",
                         ch, d.control, g_dbgPc);
        return;
    }
    if (idx > 5) return;
    uint16_t w = dmaReg(ch, idx);
    w = (addr & 1) ? uint16_t((w & 0xFF00) | val) : uint16_t((w & 0x00FF) | (val << 8));
    dmaSetReg(ch, idx, w);
}

void SystemBus::dmaTrigger(int ch, uint16_t control) {
    // MAME dma_w case 0x0a：bit15/bit11 觸發；bit10=dst 遞減、bit9=src 遞減；
    // 0xA800 特殊填充模式；bit12 word 模式；bit8 間接模式（dest 每 16 byte 回捲）
    DmaChannel &d = dma_[ch & 1];
    if (std::getenv("ACAN_DMA"))
        std::fprintf(stderr, "[dma] ch%d src=$%08X dst=$%08X count=$%04X ctrl=$%04X (pc=$%08X)\n",
                     ch, d.src, d.dst, d.count, control, g_dbgPc);
    const bool dstDec = control & 0x0400;
    const bool srcDec = control & 0x0200;
    for (uint32_t i = 0; i <= d.count; i++) {
        if (control == 0xA800) {
            // staiwbbl 開機填充：VRAM 以 word 清零、其他以 byte 複製
            if ((d.dst & 0xFE0000) == 0xF40000) {
                write16(d.dst, 0);
                d.dst += dstDec ? -2 : 2;
            } else {
                const uint8_t srcByte = d.dst & 1;
                write8(d.dst, read8(d.src + srcByte));
                d.dst += dstDec ? -1 : 1;
            }
        } else if (control & 0x1000) {
            write16(d.dst, read16(d.src));
            d.dst += dstDec ? -2 : 2;
            d.src += srcDec ? -2 : 2;
            if (control & 0x0100) {
                // 間接模式：往 $F00010-$1F 埠連寫時 dest 每 16 byte 回捲
                if ((d.dst & 0xF) == 0) d.dst -= 0x10;
            }
        } else {
            write8(d.dst, read8(d.src));
            d.dst += dstDec ? -1 : 1;
            d.src += srcDec ? -1 : 1;
        }
    }
}
