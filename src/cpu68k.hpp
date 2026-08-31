// 68k CPU：Moira 包裝
// Moira（MIT，Dirk W. Hoffmann）；時脈 10.738635 MHz（master tick /10，
// 知識庫 docs/memory-map.md §1 (a) 定案）。
#pragma once

#include "bus.hpp"
#include "state.hpp"

#include <Moira/Moira.h>

class Cpu68k : public moira::Moira {
public:
    explicit Cpu68k(SystemBus &bus) : bus_(bus) { setModel(moira::Model::M68000); }

    // IRQ ack 回呼（模擬 MAME HOLD_LINE：CPU 受理中斷後 IRQ 線自動解除）
    std::function<void(int level)> onIrqAck;

    // save state：Moira 無序列化 API，逐個複製其 protected POD 狀態
    // （M68000 不使用 iCache；lookup table 為常數不存）
    void saveState(StateWriter &w) const {
        w.put(clock);
        w.put(reg);
        w.put(queue);
        w.put(irqMode);
        w.put(ipl);
        w.put(fcl);
        w.put(fcSource);
        w.put(exception);
        w.put(cp);
        w.put(loopModeDelay);
        w.put(readBuffer);
        w.put(writeBuffer);
        w.put(flags);
    }
    void loadState(StateReader &r) {
        r.get(clock);
        r.get(reg);
        r.get(queue);
        r.get(irqMode);
        r.get(ipl);
        r.get(fcl);
        r.get(fcSource);
        r.get(exception);
        r.get(cp);
        r.get(loopModeDelay);
        r.get(readBuffer);
        r.get(writeBuffer);
        r.get(flags);
    }

protected:
    moira::u8  read8(moira::u32 addr) const override  { return bus_.read8(addr); }
    moira::u16 read16(moira::u32 addr) const override { return bus_.read16(addr); }
    moira::u32 read32(moira::u32 addr) const override { return bus_.read32(addr); }
    void write8(moira::u32 addr, moira::u8 val) const override  { bus_.write8(addr, val); }
    void write16(moira::u32 addr, moira::u16 val) const override { bus_.write16(addr, val); }
    void write32(moira::u32 addr, moira::u32 val) const override { bus_.write32(addr, val); }
    void willInterrupt(moira::u8 level) override { if (onIrqAck) onIrqAck(level); }

private:
    SystemBus &bus_;
};
