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
//   $0420 UM6619 位址埠（讀回 0 = 不忙）/ $0422 資料埠（暫存器陣列 stub）
// TODO：UM6619 合成器本體（無聲）、UM6619 timer IRQ（bit7，音樂 tempo
//   來源）、取樣 DMA IRQ（bit6）、latch $0404/$0405 3-byte 封包。
#pragma once

#include <cstdint>
#include <functional>
#include <cstdio>
#include <cstdlib>

#include <Processors/6502Mk2/6502Mk2.hpp>

namespace MOS6502Mk2 = CPU::MOS6502Mk2;

class SoundCpu {
public:
    // ram：指向 Bus 的 64KB 共享音效 RAM
    explicit SoundCpu(uint8_t *ram) : ram_(ram), cpu_(*this) {}

    void runFor(int cycles) {
        if (halted_) return;   // HALT（Reset 拉住）時不給 cycle
        cpu_.run_for(Cycles(cycles));
        // UM6619 取樣 DMA 完成 IRQ（bit6）近似：reg $16 寫 $80 啟動後延遲觸發
        // （真實時序待查證；無此 IRQ 驅動的取樣播放會卡住，sound-driver.md §4.4）
        if (sampleDmaCountdown_ > 0) {
            sampleDmaCountdown_ -= cycles;
            if (sampleDmaCountdown_ <= 0) setIrqSource(0x40, true);
        }
    }

    // $E9001C bit0：0=HALT（reset 拉住）、1=釋放。
    // CLK 的 Reset 為 level-triggered 且只在給 cycle 時才處理；HALT 期間
    // 不給 cycle，所以釋放（0→1）時要手動補跑一次 reset 序列（6-7 cycle：
    // 3 次假堆疊讀 + 取 $FFFC/$FFFD 向量）。遊戲會多次 HALT→釋放重新
    // 上傳驅動（sound-driver.md §1）。
    void setReset(bool b) {
        static const bool naive = std::getenv("ACAN_65NAIVE") != nullptr;  // 對照用
        if (naive) {
            halted_ = b;
            cpu_.template set<MOS6502Mk2::Line::Reset>(b);
            return;
        }
        if (b) {
            halted_ = true;
            cpu_.template set<MOS6502Mk2::Line::Reset>(true);
            return;
        }
        if (!halted_) return;   // 非 0→1 邊緣
        // 第一次釋放 = power-on：CLK 的 PowerOn 序列會在下一次 runFor 自然
        // 執行（deferred）；之後的釋放（遊戲重新上傳驅動，sound-driver.md §1）
        // 需手動補跑 reset 序列（7 cycle：3 假堆疊讀 + sei + 取向量）。
        // ※ 時序敏感：Speedy Dragon 的第一段 boot 若立即跑 reset 序列會卡
        //   初始化（待查證真實硬體行為）；此做法兩者皆可動。
        cpu_.template set<MOS6502Mk2::Line::Reset>(false);
        if (everReleased_) {
            cpu_.template set<MOS6502Mk2::Line::Reset>(true);
            cpu_.run_for(Cycles(7));
            cpu_.template set<MOS6502Mk2::Line::Reset>(false);
        }
        everReleased_ = true;
        halted_ = false;
    }
    bool halted() const { return halted_; }
    uint16_t getPC() const { return uint16_t(cpu_.registers().pc.full); }

    // 68k 寫 $E9000A → 65C02 IRQ 來源 bit5（sound-driver.md §4.1）
    void requestFrom68k() {
        if (std::getenv("ACAN_TRACE65"))
            std::fprintf(stderr, "[65] IRQ bit5 請求（enable=$%02X src=$%02X pc=$%04X）\n",
                         irqEnable_, irqSource_, getPC());
        setIrqSource(0x20, true);
    }

    // vblank 時 68k 端 pulse NMI（MAME scanline_cb vpos==240；驅動只 ack）
    void pulseNmi() {
        cpu_.template set<MOS6502Mk2::Line::NMI>(true);
        cpu_.template set<MOS6502Mk2::Line::NMI>(false);
    }

    // 手把狀態（16-bit active low，由 Bus 提供）
    std::function<uint16_t(int player)> getPad;

    // 65C02 寫 $040A → 通知 68k（IRQ6）
    std::function<void()> onIrqTo68k;

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
        switch (addr) {
        case 0x0402: case 0x0403: return shiftRegs_[addr - 0x0402];
        case 0x0406: return 0x00;                 // 手把 +5V presence（staiwbbl）
        case 0x0409: setIrqSource(0x10, false); return 0;  // IRQ bit4 ack
        case 0x0410: return irqEnable_;
        case 0x0411: {                            // IRQ 來源，讀取即清
            const uint8_t d = irqSource_;
            irqSource_ = 0;
            cpu_.template set<MOS6502Mk2::Line::IRQ>(false);
            return d;
        }
        case 0x0412: return 0;                    // NMI ack
        case 0x0420: return 0;                    // UM6619 狀態：bit0=0 不忙
        case 0x0422: return um6619_[um6619Addr_]; // UM6619 資料埠（stub）
        default: return ram_[addr];
        }
    }

    void writeIo(uint16_t addr, uint8_t value) {
        if (addr == 0x0300 && std::getenv("ACAN_TRACE65"))
            std::fprintf(stderr, "[65] $0300 <- $%02X (pc=$%04X)\n", value, getPC());
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
                if (lowered & (0x10 << pad)) shiftRegs_[pad] = 0;
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
            um6619_[um6619Addr_] = value;
            if (um6619Addr_ == 0x16 && (value & 0x80))
                sampleDmaCountdown_ = 3579;  // 取樣播放啟動（約 1ms 後假裝播完）
            else if (um6619Addr_ == 0x16 && value == 0x00)
                sampleDmaCountdown_ = 0;     // 停止
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
    bool everReleased_ = false;

    uint8_t irqEnable_ = 0, irqSource_ = 0;
    uint8_t shiftCtrl_ = 0;
    uint8_t shiftRegs_[2] = { 0, 0 };
    uint16_t latched_[2] = { 0xFFFF, 0xFFFF };
    uint8_t um6619Addr_ = 0;
    uint8_t um6619_[256]{};
    int sampleDmaCountdown_ = 0;   // UM6619 取樣 DMA 完成 IRQ 倒數（65C02 cycle）
};
