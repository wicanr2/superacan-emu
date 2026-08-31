// 副 CPU：WDC 65C02（CLK 核心，Thomas Harte，MIT）
// 時脈 3.579545 MHz（68k/3，docs/memory-map.md §1 (a)）。
//
// 里程碑 2 狀態：**實際執行**。卡帶程式釋放 $E9001C bit0 後開始跑
// （IPL 不動 bit0，docs/bios-68k.md §2）；遊戲會上傳音效驅動並等待
// boot OK ack（$0300=$FF，docs/sound-driver.md §1/§4），不跑會卡死。
//
// I/O $0400-$04FF 依 sound-driver.md §3 (a) 與 MAME _6502_soundmem_r/_w (b)：
//   $0402/$0403 手把 shift register（$0407 控制 latch/移位/清除）
//   $0406 手把 presence（讀 0）
//   $0409 IRQ bit4 ack（讀）
//   $040A 65C02→68k IRQ 請求（寫入觸發 68k IRQ6，經回呼通知）
//   $0410 IRQ enable / $0411 IRQ 來源（讀取即清）/ $0412 NMI ack
//   $0420 UM6619 位址埠（讀回 0 = 不忙）/ $0422 資料埠 → audio/um6619.cpp
// 已實作：UM6619 合成（PCM 16 通道 + timer IRQ bit7（音樂 tempo）+ 取樣
//   DMA IRQ bit6 雙緩衝）。
// TODO：latch $0404/$0405 3-byte 封包（IRQ bit3/bit2）。
#pragma once

#include "audio/um6619.hpp"

#include <cstdint>
#include <functional>
#include <cstdio>
#include <cstdlib>

#include <Processors/6502Mk2/6502Mk2.hpp>

extern uint64_t g_dbgFrame;  // bus.cpp；ACAN_TRACE65 日誌用

namespace MOS6502Mk2 = CPU::MOS6502Mk2;

class SoundCpu {
public:
    // ram：指向 Bus 的 64KB 共享音效 RAM
    explicit SoundCpu(uint8_t *ram) : ram_(ram), cpu_(*this), um6619_(ram) {
        um6619_.onTimerIrq = [this](bool s) { setIrqSource(0x80, s); };
        um6619_.onDmaIrq   = [this](bool s) { setIrqSource(0x40, s); };
    }

    void runFor(int cycles) {
        um6619_.tick(cycles);   // 與 65C02 同時脈域（HALT 時音訊時脈照走）
        if (halted_) return;    // HALT（Reset 拉住）時不給 cycle
        cpu_.run_for(Cycles(cycles));
        if (resetRelease_ > 0) {
            // 釋放流程保底：若 64 cycle 內沒跑到 reset 向量讀取（正常一定會），
            // 仍然放開 Reset 線（正常路徑在 readIo 讀 $FFFC 時即時放開）
            resetRelease_ -= cycles;
            if (resetRelease_ <= 0) {
                cpu_.template set<MOS6502Mk2::Line::Reset>(false);
                resetRelease_ = 0;
            }
        }
        if (std::getenv("ACAN_DBG65")) {   // 臨時除錯：每 2^16 cycle dump 暫存器
            dbgCycles_ += cycles;
            if (dbgCycles_ >= (1 << 16)) {
                dbgCycles_ = 0;
                const auto &r = cpu_.registers();
                std::fprintf(stderr, "[65] pc=$%04X a=$%02X x=$%02X y=$%02X s=$%02X irqEn=$%02X irqSrc=$%02X\n",
                             r.pc.full, r.a, r.x, r.y, r.s, irqEnable_, irqSource_);
            }
        }
    }

    // $E9001C bit0：0=HALT（reset 拉住）、1=釋放。
    // CLK 的 Reset 是 level-triggered 且只在給 cycle 時才捕捉；HALT 期間不給
    // cycle，若釋放時直接放開線，reset 請求會被「設了又清」而整個消失
    // （65C02 繼續跑舊驅動，新上傳的映像把舊 PC 底下的碼蓋掉）。
    // 所以釋放時**繼續拉住 Reset**，讓 CPU 跑完當前指令並進入 reset 序列
    // （以讀 $FFFC 向量為準）後才放開線；有 64 cycle 上限保底。
    // 遊戲會多次 HALT→釋放重新上傳驅動（sound-driver.md §1）。
    void setReset(bool b) {
        if (b) {
            halted_ = true;
            cpu_.template set<MOS6502Mk2::Line::Reset>(true);
            // I/O 暫存器隨 reset 清除（真實硬體行為；避免新驅動繼承舊 enable）
            irqEnable_ = irqSource_ = 0;
            cpu_.template set<MOS6502Mk2::Line::IRQ>(false);
            return;
        }
        if (!halted_) return;   // 非 0→1 邊緣
        halted_ = false;
        resetRelease_ = 64;     // 最多再拉 64 cycle 的 Reset
    }
    bool halted() const { return halted_; }
    uint16_t getPC() const { return uint16_t(cpu_.registers().pc.full); }

    // 68k 寫 $E9000A → 65C02 IRQ 來源 bit5（sound-driver.md §4.1）
    void requestFrom68k() {
        if (std::getenv("ACAN_TRACE65"))
            std::fprintf(stderr, "[65] f=%llu IRQ bit5 請求（enable=$%02X src=$%02X pc=$%04X）\n",
                         (unsigned long long)g_dbgFrame, irqEnable_, irqSource_, getPC());
        setIrqSource(0x20, true);
    }

    // vblank 時 68k 端 pulse NMI（MAME scanline_cb vpos==240；驅動只 ack）
    void pulseNmi() {
        cpu_.template set<MOS6502Mk2::Line::NMI>(true);
        cpu_.template set<MOS6502Mk2::Line::NMI>(false);
    }

    // UM6619 存取
    UM6619 &soundChip() { return um6619_; }

    // 手把狀態（16-bit active low，由 Bus 提供）
    std::function<uint16_t(int player)> getPad;

    // 65C02 寫 $040A → 通知 68k（IRQ6）
    std::function<void()> onIrqTo68k;

    // 68k 經 $E804xx 窗口寫 I/O 頁（MAME _68k_soundram_w 轉發行為）
    void writeFrom68k(uint16_t addr, uint8_t value) {
        if (addr == 0x0404 || addr == 0x0405) {
            // 68k→65C02 byte latch：存入並觸發 IRQ（$0404→bit3、$0405→bit2）
            const int i = addr - 0x0404;
            latch_[i] = value;
            latchFull_[i] = true;
            setIrqSource(i ? 0x04 : 0x08, true);
        }
        // 其餘 I/O 位址：RAM 已由 bus 寫入，此處不額外處理
    }

    void setIrqSource(uint8_t bit, bool state) {
        const uint8_t old = irqSource_;
        if (state) irqSource_ |= bit; else irqSource_ &= ~bit;
        if ((old ^ irqSource_) || state)
            cpu_.template set<MOS6502Mk2::Line::IRQ>((irqEnable_ & irqSource_) != 0);
    }

    // CLK BusHandler 介面
    template <MOS6502Mk2::BusOperation operation, typename AddressT>
    Cycles perform(const AddressT address, MOS6502Mk2::data_t<operation> value) {
        const uint16_t addr = uint16_t(address);
        if constexpr (MOS6502Mk2::is_read(operation)) {
            value = readIo(addr);
        } else {
            writeIo(addr, value);
        }
        return Cycles(1);
    }

private:
    uint8_t readIo(uint16_t addr) {
        if (addr == 0xFFFC && resetRelease_ > 0) {
            // 已進入 reset 序列（取向量）：即時放開 Reset 線，序列會跑完，
            // 且不會因線仍拉住而在序列結束後再觸發一次 reset
            cpu_.template set<MOS6502Mk2::Line::Reset>(false);
            resetRelease_ = 0;
            if (std::getenv("ACAN_TRACE65"))
                std::fprintf(stderr, "[65] f=%llu reset 序列完成（新 PC 向量讀取中）\n",
                             (unsigned long long)g_dbgFrame);
        }
        switch (addr) {
        case 0x0402: case 0x0403: return shiftRegs_[addr - 0x0402];
        case 0x0404: case 0x0405: {               // 68k→65C02 byte latch（空=$CD）
            const int i = addr - 0x0404;
            // 讀取 = ack 對應 latch IRQ（bit3/bit2）；各 IRQ 來源各自拉住直到
            // 專屬 ack（見 $0411 註解）
            setIrqSource(i ? 0x04 : 0x08, false);
            if (latchFull_[i]) { latchFull_[i] = false; return latch_[i]; }
            return 0xCD;                          // magic「空」值（MAME 註解/驅動實測）
        }
        case 0x0406: return 0x00;                 // 手把 +5V presence（staiwbbl）
        case 0x0409: setIrqSource(0x10, false); return 0;  // IRQ bit4 ack
        case 0x040A:                              // 讀取 = ack IRQ bit5（68k 請求；
            setIrqSource(0x20, false);            // 兩套驅動的 bit5 handler 都讀它）
            return ram_[addr];
        case 0x0410: return irqEnable_;
        case 0x0411:                              // IRQ 來源旗標（**純狀態，讀不清**）：
            return irqSource_;                    // 各 bit 由專屬 ack 清除（bit2←讀$0405、
                                                  // bit3←$0404、bit4←$0409、bit5←$040A、
                                                  // bit6←UM6619 reg $16、bit7←reg $14）。
                                                  // 兩套驅動的 dispatcher 都只分派一個
                                                  // 來源就 rti，依賴 level 重觸發補跑其餘
                                                  // （MAME 的「讀取即清全部」會丟同時發生
                                                  // 的來源，本實作不沿用，實測 Speedy 第二
                                                  // 驅動 probe 需要此行為）
        case 0x0412: return 0;                    // NMI ack
        case 0x0420: return 0;                    // UM6619 狀態：bit0=0 不忙
        case 0x0422: return um6619_.readReg(um6619Addr_);
        default: return ram_[addr];
        }
    }

    void writeIo(uint16_t addr, uint8_t value) {
        if (addr == 0x0300 && std::getenv("ACAN_TRACE65"))
            std::fprintf(stderr, "[65] f=%llu $0300 <- $%02X (pc=$%04X)\n",
                         (unsigned long long)g_dbgFrame, value, getPC());
        switch (addr) {
        case 0x0407: {  // 手把 shift 控制（MAME：bit0/1 falling=latch、
            // bit2/3 falling=移位、bit4/5 falling=清除；shift 寄存器 MSB first）
            const uint8_t lowered = shiftCtrl_ & ~value;
            shiftCtrl_ = value;
            for (int pad = 0; pad < 2; pad++) {
                if (lowered & (1 << pad))
                    latched_[pad] = getPad ? getPad(pad) : 0xFFFF;
                if (lowered & (4 << pad)) {
                    shiftRegs_[pad] = uint8_t((shiftRegs_[pad] << 1) | (latched_[pad] >> 15));
                    latched_[pad] <<= 1;
                }
                if (lowered & (0x10 << pad)) {
                    shiftRegs_[pad] = 0;
                    // 清除脈衝（probe 寫 $30）同時觸發對應 latch IRQ：
                    // 驅動的開機探測靠這個中斷讀到 $CD（空）來結束等待
                    // （Speedy 第二驅動 $F0D3 / 第一驅動 $F81B）；無此中斷
                    // 時 probe 只能靠 256×256 逾時迴圈結束（約 12 幀），
                    // 會超過 68k 端的命令等待逾時而陷入重試迴圈（實測，
                    // docs/verify-audio-input.md）。此觸發條件為功能推測。
                    setIrqSource(pad ? 0x04 : 0x08, true);
                }
            }
            return;
        }
        case 0x040A:    // 65C02→68k IRQ 請求（IRQ6）
            ram_[addr] = value;
            if (onIrqTo68k) onIrqTo68k();
            return;
        case 0x0410:
            irqEnable_ = value;
            cpu_.template set<MOS6502Mk2::Line::IRQ>((irqEnable_ & irqSource_) != 0);
            return;
        case 0x0420: um6619Addr_ = value; return;
        case 0x0422:
            um6619_.writeReg(um6619Addr_, value);
            return;
        default: ram_[addr] = value; return;
        }
    }

    struct Traits {
        static constexpr auto uses_ready_line = false;
        static constexpr auto pause_precision = MOS6502Mk2::PausePrecision::AnyCycle;
        using BusHandlerT = SoundCpu;
    };

    uint8_t *ram_;
    MOS6502Mk2::Processor<MOS6502Mk2::Model::WDC65C02, Traits> cpu_;
    bool halted_ = true;

    uint8_t irqEnable_ = 0, irqSource_ = 0;
    uint8_t shiftCtrl_ = 0;
    uint8_t shiftRegs_[2] = { 0, 0 };
    uint16_t latched_[2] = { 0xFFFF, 0xFFFF };
    uint8_t latch_[2] = { 0, 0 };        // 68k→65C02 byte latch（$0404/$0405）
    bool latchFull_[2] = { false, false };
    uint8_t um6619Addr_ = 0;
    int dbgCycles_ = 0;
    int resetRelease_ = 0;          // >0：釋放流程中，Reset 線仍拉住（等 $FFFC）
    UM6619 um6619_;                 // PCM 合成器（audio/um6619.cpp）
};
