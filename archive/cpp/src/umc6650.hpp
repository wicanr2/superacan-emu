// Deprecated C++ oracle；production 改由純 Go 獨立實作。
// UMC6650 lockout/安全晶片
// 規格出處：知識庫 docs/bios-68k.md §3、docs/memory-map.md §8（(a) 級定案）
//   $EB0D03（寫）= 內部位址埠（7-bit）
//   $EB0D01（讀寫）= 資料埠
//   內部 $20-$2F：16 byte 金鑰（umc6650.bin），唯讀
//   內部 $40-$5F：32 byte RAM，可讀寫
//   內部 $09/$0C：輸出給卡帶的 lockout 結果暫存器
// 注意：MAME umc6650.cpp 的埠角色寫反，此處以 IPL 實際用法為準。
#pragma once

#include <cstdint>
#include <cstddef>
#include <array>

#include "state.hpp"

class UMC6650 {
public:
    // 載入 16 byte 金鑰（umc6650.bin 內容）
    void loadKey(const uint8_t *key, size_t len);

    void writeAddrPort(uint8_t v) { addr_ = v & 0x7F; }
    uint8_t readDataPort() const { return mem_[addr_]; }
    void writeDataPort(uint8_t v);

    void saveState(StateWriter &w) const { w.putArray(mem_); w.put(addr_); }
    void loadState(StateReader &r) { r.getArray(mem_); r.get(addr_); }

private:
    std::array<uint8_t, 256> mem_{};
    uint8_t addr_ = 0;
};
