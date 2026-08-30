#include "umc6650.hpp"

#include <cstring>

void UMC6650::loadKey(const uint8_t *key, size_t len) {
    if (len > 16) len = 16;
    std::memcpy(&mem_[0x20], key, len);
}

void UMC6650::writeDataPort(uint8_t v) {
    // 金鑰區 $20-$2F 唯讀（docs/memory-map.md §8 (a) 確認）
    if (addr_ >= 0x20 && addr_ <= 0x2F) return;
    mem_[addr_] = v;
}
