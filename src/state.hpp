// Save state 序列化助手（見 docs/ 內各驗證文件；格式自訂，不相容 Bcan ACANRTS）
//
// 檔案格式（皆 little-endian）：
//   offset 0  magic "ACANEST1"（8 bytes）
//   u32  version（=1）
//   u32  headerSize（=96）
//   u64  romHash（FNV-1a 64-bit；非加密雜湊，僅識別用）
//   u64  payloadSize
//   其餘保留至 96 bytes
//   payload：各元件狀態依序（bus → video → sound chip → 65C02 → 68k → runner）
#pragma once

#include <cstdint>
#include <cstring>
#include <type_traits>
#include <vector>

struct StateWriter {
    std::vector<uint8_t> buf;

    template <typename T>
    void put(const T &v) {  // POD 逐位元組
        static_assert(std::is_trivially_copyable_v<T>);
        const uint8_t *p = reinterpret_cast<const uint8_t *>(&v);
        buf.insert(buf.end(), p, p + sizeof(T));
    }
    void putBytes(const uint8_t *p, size_t n) { buf.insert(buf.end(), p, p + n); }
    template <typename T, size_t N>
    void putArray(const T (&arr)[N]) { for (const auto &v : arr) put(v); }
    template <typename T, size_t N>
    void putArray(const std::array<T, N> &arr) { for (const auto &v : arr) put(v); }
};

struct StateReader {
    const uint8_t *p = nullptr;
    size_t left = 0;
    bool ok = true;

    StateReader(const uint8_t *p, size_t n) : p(p), left(n) {}

    template <typename T>
    void get(T &v) {
        static_assert(std::is_trivially_copyable_v<T>);
        if (left < sizeof(T)) { ok = false; return; }
        std::memcpy(&v, p, sizeof(T));
        p += sizeof(T); left -= sizeof(T);
    }
    void getBytes(uint8_t *dst, size_t n) {
        if (left < n) { ok = false; return; }
        std::memcpy(dst, p, n);
        p += n; left -= n;
    }
    template <typename T, size_t N>
    void getArray(T (&arr)[N]) { for (auto &v : arr) get(v); }
    template <typename T, size_t N>
    void getArray(std::array<T, N> &arr) { for (auto &v : arr) get(v); }
};

// 檔案層（main.cpp 用）：romHash 不一致時回 false 並印警告仍可由呼叫者決定是否強行
bool writeStateFile(const char *path, uint64_t romHash, const std::vector<uint8_t> &payload);
// 成功讀回 payload；romHash 不符時 *hashMismatch=true（仍回傳內容）
bool readStateFile(const char *path, uint64_t romHash, std::vector<uint8_t> &payload, bool *hashMismatch);
