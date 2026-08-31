// Deprecated C++ oracle；production 改由純 Go 獨立實作。
// UM6619 音效晶片（PCM/取樣式合成，16 通道）
// 時脈 3.579545 MHz（與 65C02 同，知識庫 docs/memory-map.md §1 (a)）。
//
// 合成/暫存器行為依 MAME `src/mame/umc/umc6619_sound.cpp`
// （BSD-3-Clause，(c) Ryan Holtz / superctr）的行為描述**重新實作**，
// 未複製其程式碼；暫存器語意另對照知識庫 docs/sound-driver.md §5 (a)。
//
// 模型（MAME (b)）：
//   - 16 通道 PCM，取樣來自 64KB 共享 sound RAM（8-bit 無號，<<8 成 16-bit）
//   - 原生抽樣率 = clock / 80（= 44744.3125 Hz），由 runner 重取樣
//   - reg $20-$2F / $30-$3F：通道 period 低/高 byte
//     （addr_increment = period << 6，16.16 固定小數點，每 native sample 遞增）
//   - reg $50-$5F：長度 = 0x40 << ((data & 0x0E) >> 1)；bit0 = one-shot；
//     bit4-7 未知（MAME 同）
//   - reg $60-$6F / $70-$7F：取樣起始位址（16-bit，實際位址 = <<6）
//   - reg $17：key 控制——低 nibble = 通道，高 nibble ≠ 0 = key-on，
//     = 0 = key-off（對應知識庫 (a)「ch|$10 key-on」）
//   - reg $90-$9F：DMA 驅動旗標（設非 0 的通道播完時觸發 DMA IRQ 並自動
//     重新 key-on——即雙緩衝取樣串流，sound-driver.md §4.4）
//   - reg $E0-$EF：音量（高 nibble 左聲道、低 nibble 右聲道，×17 擴成 8-bit）
//   - reg $11/$12：timer period（實際 period = 10 × (0x10000 - value) clocks）
//   - reg $14：timer 控制——寫入 bit7 啟動/重載 timer；bit6 設時到期觸發
//     65C02 IRQ bit7；讀取 ack timer IRQ
//   - reg $16：讀取 ack DMA IRQ（65C02 IRQ bit6）；bit6 = busy 旗標
//     （本實作：有 DMA 通道在播時 bit6=1，供驅動輪詢）
#pragma once

#include <array>
#include <cstdint>
#include <functional>

#include "state.hpp"

class UM6619 {
public:
    static constexpr int CLOCK = 3579545;             // Hz（知識庫 §1 (a)）
    static constexpr int CYCLES_PER_SAMPLE = 80;      // MAME：stream rate = clk/16/5
    static constexpr double NATIVE_RATE = double(CLOCK) / CYCLES_PER_SAMPLE;

    // ram：64KB 共享 sound RAM（65C02 空間同體，bus.soundRamData()）
    explicit UM6619(uint8_t *ram) : ram_(ram) {}

    void reset();

    // 65C02 經 $0420/$0422 的暫存器介面
    uint8_t readReg(uint8_t reg);
    void writeReg(uint8_t reg, uint8_t data);

    // 以 65C02 cycle 推進（同時脈域）；每 80 cycle 產生一個 native 立體聲樣本
    void tick(int cycles);

    // 每個 native 樣本回呼（44744.3125 Hz，已混音並 clamp 到 int16）
    std::function<void(int16_t l, int16_t r)> onSample;
    // IRQ 回呼（接 65C02 IRQ 來源 bit7/bit6）
    std::function<void(bool)> onTimerIrq;
    std::function<void(bool)> onDmaIrq;

    uint8_t rawReg(int i) const { return regs_[i & 0xFF]; }
    uint16_t activeChannels() const { return active_; }

    // save state（state.hpp；payload 內子區段）
    void saveState(StateWriter &w) const {
        w.putArray(regs_);
        for (const auto &c : ch_) w.put(c);
        w.put(active_);
        w.put(sampleAcc_);
        w.put(timerCount_);
        w.put(timerIrq_);
        w.put(dmaIrq_);
    }
    void loadState(StateReader &r) {
        r.getArray(regs_);
        for (auto &c : ch_) r.get(c);
        r.get(active_);
        r.get(sampleAcc_);
        r.get(timerCount_);
        r.get(timerIrq_);
        r.get(dmaIrq_);
    }

private:
    struct Channel {
        uint16_t pitch = 0;          // period（16-bit）
        uint32_t addrIncrement = 0;  // pitch << 6（16.16 固定小數點）
        uint16_t startAddr = 0;      // 單位 0x40 bytes
        uint16_t length = 0;         // bytes
        uint16_t currAddr = 0;       // sound RAM byte 位址
        uint16_t endAddr = 0;
        uint32_t frac = 0;           // 16.16 小數累積
        uint8_t register9 = 0;       // $9x：DMA 驅動旗標
        uint8_t volumeL = 0, volumeR = 0;
        bool oneShot = false;
    };

    void keyOn(int ch);
    void mixSample();

    uint8_t *ram_;
    std::array<uint8_t, 256> regs_{};
    std::array<Channel, 16> ch_{};
    uint16_t active_ = 0;

    int sampleAcc_ = 0;              // → 80 cycle 一個樣本
    int timerCount_ = -1;            // timer 倒數（clocks；<0 = 停止）
    bool timerIrq_ = false;
    bool dmaIrq_ = false;
};
