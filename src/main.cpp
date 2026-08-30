// superacan-emu headless runner（里程碑 1：68k BIOS IPL 驗證）
//
// 用法：superacan-emu --bios <dir> --rom <file> [--trace N] [--instructions N]
//
// 驗證目標（知識庫 docs/bios-68k.md）：
//   IPL $400 進入 → $40A UMC6650 交握 → $55A 卡帶授權比對 →
//   $604 設 $E9001C bit1/bit3 關 overlay → JMP (A0) 跳卡帶向量入口。
#include "bus.hpp"
#include "cpu68k.hpp"
#include "cpu65c02.hpp"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fstream>
#include <string>
#include <vector>

namespace {

std::vector<uint8_t> readFile(const std::string &path) {
    std::ifstream f(path, std::ios::binary);
    if (!f) {
        std::fprintf(stderr, "錯誤：無法開啟 %s\n", path.c_str());
        std::exit(1);
    }
    return std::vector<uint8_t>(std::istreambuf_iterator<char>(f), {});
}

// 16-bit word-swap 還原（ROM/BIOS 檔案格式，docs/bios-rom-format.md §1.1/§2.1）
void wordSwap(std::vector<uint8_t> &v) {
    for (size_t i = 0; i + 1 < v.size(); i += 2) std::swap(v[i], v[i + 1]);
}

// IPL 關鍵位址（docs/bios-68k.md §2-§4，對 word-swap 還原後映像）
constexpr uint32_t PC_LOCKOUT_START = 0x40A;   // UMC6650 檢查常式入口
constexpr uint32_t PC_LOCKOUT_DONE  = 0x55A;   // lockout 結果寫出、進入授權比對
constexpr uint32_t PC_LICENSE_DONE  = 0x5F4;   // 授權比對全數通過
constexpr uint32_t PC_HANDOVER      = 0xF80604; // JMP $F80604 後於高區視圖執行   // 關 overlay、轉交控制權

void usage(const char *argv0) {
    std::fprintf(stderr,
        "用法：%s --bios <dir> --rom <file> [--trace N] [--instructions N]\n"
        "  --bios <dir>        內含 internal_68k.bin、umc6650.bin\n"
        "                      （可選 internal_6502_1/2.bin 取樣資料）\n"
        "  --rom <file>        卡帶 ROM（word-swap raw binary，無標頭）\n"
        "  --trace N           以反組譯 log 前 N 條指令\n"
        "  --instructions N    到達卡帶入口後再執行的指令數（預設 5000）\n",
        argv0);
}

} // namespace

int main(int argc, char **argv) {
    std::string biosDir, romPath;
    long traceCount = 0, postEntryInstrs = 5000;
    for (int i = 1; i < argc; i++) {
        std::string a = argv[i];
        auto next = [&](const char *name) -> std::string {
            if (++i >= argc) { std::fprintf(stderr, "錯誤：%s 缺參數\n", name); std::exit(1); }
            return argv[i];
        };
        if (a == "--bios") biosDir = next("--bios");
        else if (a == "--rom") romPath = next("--rom");
        else if (a == "--trace") traceCount = std::atol(next("--trace").c_str());
        else if (a == "--instructions") postEntryInstrs = std::atol(next("--instructions").c_str());
        else { usage(argv[0]); return 1; }
    }
    if (biosDir.empty() || romPath.empty()) { usage(argv[0]); return 1; }

    SystemBus bus;

    // BIOS：68k IPL（必備）、UMC6650 金鑰（必備）、6502 取樣資料（可選，
    // 開機複製進 sound RAM $0000-$3FFF，docs/bios-65c02.md）
    auto ipl = readFile(biosDir + "/internal_68k.bin");
    if (ipl.size() != 4096) { std::fprintf(stderr, "錯誤：internal_68k.bin 應為 4096 bytes\n"); return 1; }
    wordSwap(ipl);
    bus.loadIpl(ipl.data(), ipl.size());

    auto key = readFile(biosDir + "/umc6650.bin");
    if (key.size() != 16) { std::fprintf(stderr, "錯誤：umc6650.bin 應為 16 bytes\n"); return 1; }
    bus.lockout().loadKey(key.data(), key.size());

    for (int i = 1; i <= 2; i++) {
        std::string p = biosDir + "/internal_6502_" + std::to_string(i) + ".bin";
        std::ifstream f(p, std::ios::binary);
        if (f) {
            std::vector<uint8_t> d(std::istreambuf_iterator<char>(f), {});
            bus.loadSoundRam(d.data(), d.size(), uint32_t(i - 1) * 0x2000);
        }
    }

    auto rom = readFile(romPath);
    if (rom.empty()) { std::fprintf(stderr, "錯誤：ROM 為空\n"); return 1; }
    wordSwap(rom);
    bus.loadRom(std::move(rom));

    SoundCpu soundCpu(bus.soundRamData());
    soundCpu.setReset(true);  // IPL 不動 $E9001C bit0：65C02 保持 HALT

    Cpu68k cpu(bus);

    // overlay 狀態變化 log
    bus.onControlWrite = [](uint16_t oldV, uint16_t newV) {
        std::printf("[event] $E9001C: $%04X -> $%04X", oldV, newV);
        if ((newV & 0x0002) && !(oldV & 0x0002)) std::printf("  [bit1：關閉低區 IPL overlay]");
        if ((newV & 0x0008) && !(oldV & 0x0008)) std::printf("  [bit3：關閉高區 IPL overlay]");
        std::printf("\n");
    };

    cpu.reset();
    std::printf("[boot] reset：SSP=$%08X PC=$%08X（IPL 向量表）\n",
                cpu.getA(7), cpu.getPC());

    const uint32_t expectedEntry = bus.cartVector(1);
    std::printf("[info] 卡帶向量表入口 PC=$%08X SSP=$%08X\n", expectedEntry, bus.cartVector(0));

    enum class Phase { Ipl, CartEntry, Done };
    Phase phase = Phase::Ipl;
    bool lockoutLogged = false, licenseLogged = false, handoverLogged = false;
    long traced = 0, postRun = 0;
    const long maxIplInstrs = 2000000;  // 保底：IPL 正常僅需數千條
    long total = 0;

    char dasm[128];
    while (phase != Phase::Done) {
        const uint32_t pc = cpu.getPC();

        if (traceCount > 0 && traced < traceCount) {
            cpu.disassemble(dasm, pc);
            std::printf("[trace] $%08X  %s\n", pc, dasm);
            traced++;
        }

        if (phase == Phase::Ipl) {
            if (!lockoutLogged && pc == PC_LOCKOUT_DONE) {
                std::printf("[event] UMC6650 交握通過（IPL $%03X：lockout 結果 $09/$0C 已寫出）\n",
                            PC_LOCKOUT_DONE);
                lockoutLogged = true;
            }
            if (!licenseLogged && pc == PC_LICENSE_DONE) {
                std::printf("[event] 卡帶授權比對通過（IPL $%03X）\n", PC_LICENSE_DONE);
                licenseLogged = true;
            }
            if (!handoverLogged && pc == PC_HANDOVER) {
                std::printf("[event] IPL 轉交控制權（IPL $%03X：關 overlay → JMP (A0)）\n",
                            PC_HANDOVER);
                handoverLogged = true;
            }
            if (handoverLogged && pc == expectedEntry) {
                std::printf("[event] *** 進入卡帶入口 PC=$%08X ***\n", pc);
                phase = Phase::CartEntry;
            } else if (++total > maxIplInstrs) {
                std::fprintf(stderr, "錯誤：IPL 執行 %ld 條仍未到卡帶入口，停止\n", total);
                return 2;
            }
        } else if (phase == Phase::CartEntry) {
            if (++postRun >= postEntryInstrs) phase = Phase::Done;
        }

        try {
            cpu.execute();
        } catch (const std::exception &e) {
            std::fprintf(stderr, "[fault] PC=$%08X 例外：%s\n", pc, e.what());
            return 3;
        }
    }

    std::printf("[done] 到達卡帶入口後再執行 %ld 條指令，無 bus fault\n", postRun);
    std::printf("[done] 最終 PC=$%08X\n", cpu.getPC());
    return 0;
}
