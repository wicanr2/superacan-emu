// Deprecated C++ oracle；production 改由純 Go 獨立實作。
// Save state 檔案讀寫（格式見 state.hpp）
#include "state.hpp"

#include <cstdio>

static constexpr char MAGIC[8] = { 'A', 'C', 'A', 'N', 'E', 'S', 'T', '1' };
static constexpr uint32_t VERSION = 1;
static constexpr uint32_t HEADER_SIZE = 96;

bool writeStateFile(const char *path, uint64_t romHash, const std::vector<uint8_t> &payload) {
    FILE *f = std::fopen(path, "wb");
    if (!f) return false;
    uint8_t hdr[HEADER_SIZE] = {};
    std::memcpy(hdr, MAGIC, 8);
    auto put32 = [&](int off, uint32_t v) { std::memcpy(hdr + off, &v, 4); };
    auto put64 = [&](int off, uint64_t v) { std::memcpy(hdr + off, &v, 8); };
    put32(8, VERSION);
    put32(12, HEADER_SIZE);
    put64(16, romHash);
    put64(24, uint64_t(payload.size()));
    const bool ok = std::fwrite(hdr, 1, HEADER_SIZE, f) == HEADER_SIZE &&
                    (payload.empty() || std::fwrite(payload.data(), 1, payload.size(), f) == payload.size());
    std::fclose(f);
    return ok;
}

bool readStateFile(const char *path, uint64_t romHash, std::vector<uint8_t> &payload, bool *hashMismatch) {
    if (hashMismatch) *hashMismatch = false;
    FILE *f = std::fopen(path, "rb");
    if (!f) return false;
    uint8_t hdr[HEADER_SIZE];
    if (std::fread(hdr, 1, HEADER_SIZE, f) != HEADER_SIZE || std::memcmp(hdr, MAGIC, 8) != 0) {
        std::fclose(f);
        return false;
    }
    uint32_t version, headerSize;
    uint64_t fileHash, payloadSize;
    std::memcpy(&version, hdr + 8, 4);
    std::memcpy(&headerSize, hdr + 12, 4);
    std::memcpy(&fileHash, hdr + 16, 8);
    std::memcpy(&payloadSize, hdr + 24, 8);
    if (version != VERSION || headerSize > 4096 || payloadSize > (64ull << 20)) {
        std::fclose(f);
        return false;
    }
    if (headerSize > HEADER_SIZE) std::fseek(f, long(headerSize - HEADER_SIZE), SEEK_CUR);
    payload.resize(payloadSize);
    const bool ok = payloadSize == 0 || std::fread(payload.data(), 1, payloadSize, f) == payloadSize;
    std::fclose(f);
    if (hashMismatch && fileHash != romHash) *hashMismatch = true;
    return ok;
}
