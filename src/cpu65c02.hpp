// 副 CPU：WDC 65C02（CLK 核心，Thomas Harte，MIT）
// 時脈 3.579545 MHz（master tick /30，docs/memory-map.md §1 (a)）。
//
// 本里程碑狀態：**介面 stub**——已把 CLK 的 6502Mk2（Model::WDC65C02）
// 整合進 build，BusHandler 映射共享音效 RAM（68k 端 $E80000-$E8FFFF，
// docs/memory-map.md §5）與 I/O $0400-$04FF（暫以 stub 讀 0），但
// run() 不被呼叫：IPL 全程未解除 65C02 的 HALT（$E9001C bit0 不動，
// docs/bios-68k.md §2），開機後由卡帶程式自行上傳代碼並釋放。
// I/O 暫存器語意見 docs/memory-map.md §5 表（手把/IRQ/UM6619 間接埠）。
#pragma once

#include <cstdint>

#include <Processors/6502Mk2/6502Mk2.hpp>

namespace MOS6502Mk2 = CPU::MOS6502Mk2;

class SoundCpu {
public:
    // ram：指向 Bus 的 64KB 共享音效 RAM
    explicit SoundCpu(uint8_t *ram) : ram_(ram), cpu_(*this) {}

    // 本里程碑不執行（保持 HALT/reset）
    void runFor(int /*cycles*/) {}

    void setReset(bool b) { cpu_.template set<MOS6502Mk2::Line::Reset>(b); }

    // CLK BusHandler 介面
    template <MOS6502Mk2::BusOperation operation, typename AddressT>
    Cycles perform(const AddressT address, MOS6502Mk2::data_t<operation> value) {
        const uint16_t addr = uint16_t(address);
        if constexpr (MOS6502Mk2::is_read(operation)) {
            if (addr >= 0x0400 && addr < 0x0500) value = 0;  // I/O stub（memory-map.md §5）
            else value = ram_[addr];
        } else {
            ram_[addr] = value;
        }
        return Cycles(1);
    }

private:
    struct Traits {
        static constexpr auto uses_ready_line = false;
        static constexpr auto pause_precision = MOS6502Mk2::PausePrecision::AnyCycle;
        using BusHandlerT = SoundCpu;
    };

    uint8_t *ram_;
    MOS6502Mk2::Processor<MOS6502Mk2::Model::WDC65C02, Traits> cpu_;
};
