#include "bus.hpp"

#include <algorithm>
#include <cstring>

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
    if (addr >= 0xE80000 && addr < 0xE90000) return soundRam_[addr & 0xFFFF];
    if (addr >= 0xE90000 && addr < 0xE91000) {
        // $E9001C/$E9001D：sound CPU / lockout 控制暫存器（16-bit）
        if (addr == 0xE9001C) return uint8_t(ctrl_ >> 8);
        if (addr == 0xE9001D) return uint8_t(ctrl_);
        if (addr == 0xE90B3C) return uint8_t(e90b3c_ >> 8);
        if (addr == 0xE90B3D) return uint8_t(e90b3c_);
        return 0;  // UM6619 host port / DMA stub：讀 0
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
    if (addr >= 0xF00000 && addr < 0xF00200) return 0;  // UM6618 stub：讀 0
    if (addr >= 0xF00200 && addr < 0xF00400) {
        uint16_t w = palette_[(addr & 0x1FF) >> 1];
        return (addr & 1) ? uint8_t(w) : uint8_t(w >> 8);
    }
    if (addr >= 0xF40000 && addr < 0xF60000) return vram_[addr & 0x1FFFF];
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
    if (addr >= 0xE80000 && addr < 0xE90000) { soundRam_[addr & 0xFFFF] = val; return; }
    if (addr >= 0xE90000 && addr < 0xE91000) {
        if (addr == 0xE90B3C) { e90b3c_ = uint16_t((e90b3c_ & 0x00FF) | (val << 8)); return; }
        if (addr == 0xE90B3D) { e90b3c_ = uint16_t((e90b3c_ & 0xFF00) | val); return; }
        if (addr == 0xE9001C || addr == 0xE9001D) {
            uint16_t oldV = ctrl_;
            if (addr == 0xE9001C) ctrl_ = uint16_t((ctrl_ & 0x00FF) | (val << 8));
            else                  ctrl_ = uint16_t((ctrl_ & 0xFF00) | val);
            if (ctrl_ != oldV && onControlWrite) onControlWrite(oldV, ctrl_);
            return;
        }
        return;  // UM6619 host port / DMA stub：寫入忽略
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
    if (addr >= 0xF00000 && addr < 0xF00200) return;  // UM6618 stub：寫入忽略
    if (addr >= 0xF00200 && addr < 0xF00400) {
        uint16_t &w = palette_[(addr & 0x1FF) >> 1];
        w = (addr & 1) ? uint16_t((w & 0xFF00) | val) : uint16_t((w & 0x00FF) | (val << 8));
        return;
    }
    if (addr >= 0xF40000 && addr < 0xF60000) { vram_[addr & 0x1FFFF] = val; return; }
    if (addr >= 0xFC0000) { wram_[addr & 0xFFFF] = val; return; }
    // 其餘（含 $F00400-$F3FFFF、$F60000-$F7FFFF no-op 區段）：忽略
}

void SystemBus::write16(uint32_t addr, uint16_t val) {
    write8(addr, uint8_t(val >> 8));
    write8(addr + 1, uint8_t(val));
}
