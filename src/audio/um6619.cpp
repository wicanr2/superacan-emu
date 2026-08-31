// UM6619 音效晶片實作。行為依 MAME umc6619_sound.cpp（BSD-3-Clause，
// Ryan Holtz / superctr）重新實作，未複製程式碼；暫存器語意對照知識庫
// docs/sound-driver.md §5 (a)。
#include "um6619.hpp"

#include <algorithm>

void UM6619::reset() {
    regs_.fill(0);
    for (auto &c : ch_) c = Channel{};
    active_ = 0;
    sampleAcc_ = 0;
    timerCount_ = -1;
    timerIrq_ = dmaIrq_ = false;
}

void UM6619::keyOn(int ch) {
    Channel &c = ch_[ch & 15];
    c.currAddr = uint16_t(c.startAddr << 6);
    c.endAddr = uint16_t(c.currAddr + c.length);
    c.frac = 0;
    active_ |= uint16_t(1 << (ch & 15));
}

uint8_t UM6619::readReg(uint8_t reg) {
    if (reg == 0x14) {  // 讀取 ack timer IRQ（MAME read case 0x14）
        timerIrq_ = false;
        if (onTimerIrq) onTimerIrq(false);
    } else if (reg == 0x16) {  // 讀取 ack DMA IRQ；bit6 = DMA busy（供輪詢）
        dmaIrq_ = false;
        if (onDmaIrq) onDmaIrq(false);
        // 覆寫 bit6：有 DMA 驅動通道在播 → busy
        bool busy = false;
        for (int i = 0; i < 16; i++)
            if ((active_ & (1 << i)) && ch_[i].register9) busy = true;
        return uint8_t((regs_[0x16] & ~0x40) | (busy ? 0x40 : 0));
    }
    return regs_[reg];
}

void UM6619::writeReg(uint8_t reg, uint8_t data) {
    regs_[reg] = data;
    const uint8_t upper = reg >> 4, lower = reg & 0x0F;
    switch (upper) {
    case 0x1:
        if (lower == 0x4) {
            // timer 控制：bit7 啟動/重載（period = 10 × (0x10000 - $12$11)）
            if (data & 0x80) {
                const uint16_t period = uint16_t((regs_[0x12] << 8) | regs_[0x11]);
                timerCount_ = 10 * (0x10000 - period);
            }
        } else if (lower == 0x7) {
            // key on/off：低 nibble=通道，高 nibble≠0=key-on
            if (data & 0xF0) keyOn(lower);
            else active_ &= ~uint16_t(1 << lower);
        }
        break;
    case 0x2:  // period 低 byte
        ch_[lower].pitch = uint16_t((ch_[lower].pitch & 0xFF00) | data);
        ch_[lower].addrIncrement = uint32_t(ch_[lower].pitch) << 6;
        break;
    case 0x3:  // period 高 byte
        ch_[lower].pitch = uint16_t((ch_[lower].pitch & 0x00FF) | (data << 8));
        ch_[lower].addrIncrement = uint32_t(ch_[lower].pitch) << 6;
        break;
    case 0x5:  // 波形長度 + one-shot
        ch_[lower].length = uint16_t(0x40 << ((data & 0x0E) >> 1));
        ch_[lower].oneShot = data & 1;
        break;
    case 0x6:  // 起始位址高 byte（單位 0x40）
        ch_[lower].startAddr = uint16_t((ch_[lower].startAddr & 0x00FF) | (data << 8));
        break;
    case 0x7:  // 起始位址低 byte
        ch_[lower].startAddr = uint16_t((ch_[lower].startAddr & 0xFF00) | data);
        break;
    case 0x9:  // DMA 驅動旗標（$FF = 雙緩衝串流通道）
        ch_[lower].register9 = data;
        break;
    case 0xE: {  // 音量：高 nibble 左、低 nibble 右（×17 擴 8-bit）
        ch_[lower].volumeL = uint8_t((data & 0xF0) | (data >> 4));
        ch_[lower].volumeR = uint8_t((data & 0x0F) | (data << 4));
        break;
    }
    default:
        break;  // $Ax-$Dx envelope 等：MAME 亦未實作
    }
}

void UM6619::tick(int cycles) {
    // timer（period 到 → 若 reg $14 bit6 設則觸發 65C02 IRQ bit7，並重載）
    if (timerCount_ >= 0) {
        timerCount_ -= cycles;
        while (timerCount_ < 0) {
            const uint16_t period = uint16_t((regs_[0x12] << 8) | regs_[0x11]);
            timerCount_ += 10 * (0x10000 - period);
            if (regs_[0x14] & 0x40) {
                timerIrq_ = true;
                if (onTimerIrq) onTimerIrq(true);
            }
        }
    }
    // 取樣：每 80 cycle 一個 native 樣本
    sampleAcc_ += cycles;
    while (sampleAcc_ >= CYCLES_PER_SAMPLE) {
        sampleAcc_ -= CYCLES_PER_SAMPLE;
        mixSample();
    }
}

void UM6619::mixSample() {
    int32_t mixL = 0, mixR = 0;
    for (int i = 0; i < 16 && active_; i++) {
        if (!(active_ & (1 << i))) continue;
        Channel &c = ch_[i];
        const int16_t s = int16_t((int(ram_[c.currAddr]) - 128) << 8);  // 無號 8-bit → 16-bit
        mixL += (s * c.volumeL) >> 8;
        mixR += (s * c.volumeR) >> 8;
        c.frac += c.addrIncrement;
        c.currAddr = uint16_t(c.currAddr + (c.frac >> 16));
        c.frac &= 0xFFFF;
        if (c.currAddr >= c.endAddr) {
            if (c.register9) {
                // DMA 驅動（雙緩衝）：播完觸發 65C02 IRQ bit6 並重新 key-on
                dmaIrq_ = true;
                if (onDmaIrq) onDmaIrq(true);
                keyOn(i);
            } else if (c.oneShot) {
                active_ &= ~uint16_t(1 << i);
            } else {
                c.currAddr = uint16_t(c.currAddr - c.length);  // loop
            }
        }
    }
    if (onSample)
        // >>1 留 headroom：16 通道滿幅疊加會超過 int16（MAME 由 stream 正規化，
        // 此處直接 clamp 前縮半，避免爆音）
        onSample(int16_t(std::clamp(mixL >> 1, -32768, 32767)),
                 int16_t(std::clamp(mixR >> 1, -32768, 32767)));
}
