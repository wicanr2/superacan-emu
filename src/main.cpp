// superacan-emu runner（里程碑 2：UM6618 繪圖 + SDL2 視窗 / headless 驗證）
//
// 用法：
//   superacan-emu --bios <dir> --rom <file> [--trace N] [--instructions N]
//                 [--headless] [--frames N] [--screenshot <file.bmp>]
//
// 時序（知識庫 docs/memory-map.md §1 (a) + MAME machine_config (b)）：
//   68k = 10.738635 MHz；pixel clock = U13/10（256 模式，342 線寬）或
//   U13/8（320 模式，455 線寬）；262 線/幀。每線 68k cycle 數：
//   256 模式 = 342 × 2 = 684；320 模式 = 455 × 1.6 = 728。
// 中斷（§6 (b)）：vblank（線 240）→ 68k IRQ7（irq mask $E90010 bit7，
//   讀 $F00000 清除）；raster（每條可視線）→ IRQ4（mask bit4，單次脈衝）。
#include "bus.hpp"
#include "cpu68k.hpp"
#include "cpu65c02.hpp"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <fstream>
#include <string>
#include <vector>

extern uint32_t g_dbgPc;  // bus.cpp 的除錯 watchpoint 用

#ifndef NO_SDL
#include <SDL.h>
#endif

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
constexpr uint32_t PC_LOCKOUT_DONE  = 0x55A;    // lockout 結果寫出、進入授權比對
constexpr uint32_t PC_LICENSE_DONE  = 0x5F4;    // 授權比對全數通過
constexpr uint32_t PC_HANDOVER      = 0xF80604; // 關 overlay、轉交控制權

void usage(const char *argv0) {
    std::fprintf(stderr,
        "用法：%s --bios <dir> --rom <file> [選項]\n"
        "  --bios <dir>        內含 internal_68k.bin、umc6650.bin\n"
        "                      （可選 internal_6502_1/2.bin 取樣資料）\n"
        "  --rom <file>        卡帶 ROM（word-swap raw binary，無標頭）\n"
        "  --trace N           以反組譯 log 前 N 條指令\n"
        "  --instructions N    headless 且無 --frames 時，到卡帶入口後再跑的指令數（預設 5000）\n"
        "  --headless          不開 SDL2 視窗\n"
        "  --frames N          跑 N 幀後結束（預設：SDL 模式跑到關窗）\n"
        "  --screenshot <f>    結束時把最後一幀寫成 24-bit BMP\n",
        argv0);
}

// 24-bit BMP（bottom-up）
bool writeBmp(const std::string &path, const std::vector<uint32_t> &rgbx, int w, int h) {
    std::ofstream f(path, std::ios::binary);
    if (!f) return false;
    const uint32_t rowBytes = uint32_t(w) * 3;
    const uint32_t imgSize = rowBytes * uint32_t(h);
    const uint32_t fileSize = 54 + imgSize;
    auto put32 = [&](uint32_t v) { f.put(char(v)); f.put(char(v >> 8)); f.put(char(v >> 16)); f.put(char(v >> 24)); };
    auto put16 = [&](uint16_t v) { f.put(char(v)); f.put(char(v >> 8)); };
    f.write("BM", 2);
    put32(fileSize); put32(0); put32(54);
    put32(40); put32(uint32_t(w)); put32(uint32_t(h)); put16(1); put16(24);
    put32(0); put32(imgSize); put32(2835); put32(2835); put32(0); put32(0);
    for (int y = h - 1; y >= 0; y--)
        for (int x = 0; x < w; x++) {
            const uint32_t p = rgbx[size_t(y) * w + x];  // 0xFFRRGGBB
            f.put(char(p)); f.put(char(p >> 8)); f.put(char(p >> 16));  // B,G,R
        }
    return bool(f);
}

} // namespace

int main(int argc, char **argv) {
    std::string biosDir, romPath, screenshotPath;
    long traceCount = 0, postEntryInstrs = 5000, frameLimit = -1;
    bool headless = false;
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
        else if (a == "--headless") headless = true;
        else if (a == "--frames") frameLimit = std::atol(next("--frames").c_str());
        else if (a == "--screenshot") screenshotPath = next("--screenshot");
        else { usage(argv[0]); return 1; }
    }
    if (biosDir.empty() || romPath.empty()) { usage(argv[0]); return 1; }

#ifndef NO_SDL
    if (!headless && SDL_Init(SDL_INIT_VIDEO) != 0) {
        std::fprintf(stderr, "警告：SDL_Init 失敗（%s），改為 headless\n", SDL_GetError());
        headless = true;
    }
#else
    headless = true;
#endif
    if (headless && frameLimit < 0 && postEntryInstrs <= 0) postEntryInstrs = 5000;

    SystemBus bus;

    // BIOS：68k IPL（必備）、UMC6650 金鑰（必備）、6502 取樣資料（可選）
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
    soundCpu.setReset(true);  // IPL 不動 bit0：65C02 保持 HALT，等卡帶釋放
    soundCpu.getPad = [&bus](int p) { return bus.pad(p); };
    bool soundIrq6 = false;   // 65C02→68k mailbox IRQ6（脈衝）
    soundCpu.onIrqTo68k = [&soundIrq6] { soundIrq6 = true; };
    bus.onSoundIrqRequest = [&soundCpu] { soundCpu.requestFrom68k(); };

    // UM6618 sprite DMA 需要回寫整個位址空間
    bus.video().busRead16 = [&bus](uint32_t a) { return bus.read16(a); };
    bus.video().busWrite16 = [&bus](uint32_t a, uint16_t v) { bus.write16(a, v); };

    Cpu68k cpu(bus);

    bus.onControlWrite = [&soundCpu](uint16_t oldV, uint16_t newV) {
        // 只 log bit0-3 變化（卡帶會頻繁翻其他位元，避免洗版）
        if ((newV ^ oldV) & 0x0F)
            std::printf("[event] $E9001C: $%04X -> $%04X%s%s%s\n", oldV, newV,
                        ((newV ^ oldV) & 1) ? ((newV & 1) ? "  [bit0：釋放 65C02]" : "  [bit0：HALT 65C02]") : "",
                        ((newV & 2) && !(oldV & 2)) ? "  [bit1：關低區 overlay]" : "",
                        ((newV & 8) && !(oldV & 8)) ? "  [bit3：關高區 overlay]" : "");
        if ((newV ^ oldV) & 1) soundCpu.setReset(!(newV & 1));  // bit0：65C02 reset
    };

    cpu.reset();
    std::printf("[boot] reset：SSP=$%08X PC=$%08X（IPL 向量表）\n", cpu.getA(7), cpu.getPC());

    const uint32_t expectedEntry = bus.cartVector(1);
    std::printf("[info] 卡帶向量表入口 PC=$%08X SSP=$%08X\n", expectedEntry, bus.cartVector(0));

#ifndef NO_SDL
    SDL_Window *window = nullptr;
    SDL_Renderer *renderer = nullptr;
    SDL_Texture *texture = nullptr;
    if (!headless) {
        window = SDL_CreateWindow("superacan-emu", SDL_WINDOWPOS_CENTERED, SDL_WINDOWPOS_CENTERED,
                                  UM6618::WIDTH * 3, UM6618::HEIGHT * 3, SDL_WINDOW_RESIZABLE);
        renderer = window ? SDL_CreateRenderer(window, -1, SDL_RENDERER_ACCELERATED | SDL_RENDERER_PRESENTVSYNC) : nullptr;
        texture = renderer ? SDL_CreateTexture(renderer, SDL_PIXELFORMAT_ARGB8888,
                                               SDL_TEXTUREACCESS_STREAMING, UM6618::WIDTH, UM6618::HEIGHT) : nullptr;
        if (!texture) {
            std::fprintf(stderr, "警告：SDL 視窗建立失敗，改為 headless\n");
            headless = true;
        }
    }
#endif

    // ---- 主迴圈：IPL 階段 → 幀時序階段 ----
    bool atCartEntry = false, handoverLogged = false;
    bool lockoutLogged = false, licenseLogged = false;
    long traced = 0, postRun = 0;
    const long maxIplInstrs = 2000000;
    long total = 0;
    int vpos = 0;
    int64_t lineCycles = 0;
    int soundCycleAcc = 0;   // 65C02 = 68k/3（§1 (a)）
    int currentIpl = 0;
    bool quit = false;

    char dasm[128];
    auto applyIrq = [&]() {
        // vblank（IRQ7）> mailbox（IRQ6）> raster（IRQ4）；mask 在 $E90010（§6 (b)）
        int lvl = 0;
        if (bus.video().vblankPending()) lvl = 7;
        else if (soundIrq6) lvl = 6;
        else if (bus.video().irq5Pending()) lvl = 5;
        else if (bus.video().rasterActive()) lvl = 4;
        if (lvl != currentIpl) { cpu.setIPL(uint8_t(lvl)); currentIpl = lvl; }
    };
    // HOLD_LINE 語意：CPU 受理中斷（Moira willInterrupt）即解除對應 IRQ 線。
    // 這很重要：level 7 在 68k 上是 NMI 式邊緣觸發，遊戲用 STOP #$2700 等
    // vblank；若受理後線不解除，會反覆觸發把 CPU 鎖死在中斷進入循環。
    long irqAckCount[8] = {};
    cpu.onIrqAck = [&](int level) {
        irqAckCount[level & 7]++;
        if (level == 7) bus.video().clearVblankPulse();
        else if (level == 6) soundIrq6 = false;
        else if (level == 5) bus.video().clearIrq5();
        else if (level == 4) bus.video().clearRaster();
        applyIrq();
    };

    while (!quit) {
        const uint32_t pc = cpu.getPC();
        g_dbgPc = pc;

        if (traceCount > 0 && traced < traceCount) {
            cpu.disassemble(dasm, pc);
            std::printf("[trace] $%08X  %s\n", pc, dasm);
            traced++;
        }

        if (!atCartEntry) {
            if (!lockoutLogged && pc == PC_LOCKOUT_DONE) {
                std::printf("[event] UMC6650 交握通過（IPL $%03X）\n", PC_LOCKOUT_DONE);
                lockoutLogged = true;
            }
            if (!licenseLogged && pc == PC_LICENSE_DONE) {
                std::printf("[event] 卡帶授權比對通過（IPL $%03X）\n", PC_LICENSE_DONE);
                licenseLogged = true;
            }
            if (!handoverLogged && pc == PC_HANDOVER) {
                std::printf("[event] IPL 轉交控制權（IPL $%03X：關 overlay → JMP (A0)）\n", PC_HANDOVER);
                handoverLogged = true;
            }
            if (handoverLogged && pc == expectedEntry) {
                std::printf("[event] *** 進入卡帶入口 PC=$%08X ***\n", pc);
                atCartEntry = true;
            } else if (++total > maxIplInstrs) {
                std::fprintf(stderr, "錯誤：IPL 執行 %ld 條仍未到卡帶入口，停止\n", total);
                return 2;
            }
        } else if (frameLimit < 0 && headless && ++postRun >= postEntryInstrs) {
            break;  // 舊行為：入口後固定指令數
        }

        try {
            const int64_t before = cpu.getClock();
            cpu.execute();
            const int64_t delta = cpu.getClock() - before;
            lineCycles += delta;
            soundCycleAcc += int(delta);
            soundCpu.runFor(soundCycleAcc / 3);  // 65C02 = 68k/3
            soundCycleAcc %= 3;
        } catch (const std::exception &e) {
            std::fprintf(stderr, "[fault] PC=$%08X 例外：%s\n", pc, e.what());
            return 3;
        }

        if (!atCartEntry) continue;

        // clock 凍結偵測（CPU 停擺 = 模擬器 bug；dump 最近 64 個 PC 後退出）
        {
            static uint32_t ring[64]; static int ringPos = 0;
            static long sinceClock = 0; static int64_t lastClock = 0;
            ring[ringPos] = cpu.getPC(); ringPos = (ringPos + 1) & 63;
            if (cpu.getClock() != lastClock) { lastClock = cpu.getClock(); sinceClock = 0; }
            else if (++sinceClock == 200000) {
                std::fprintf(stderr, "[fault] 68k clock 凍結 @%lld，最近 PC 軌跡：\n", (long long)lastClock);
                for (int k = 0; k < 64; k++)
                    std::fprintf(stderr, "  $%08X%s", ring[(ringPos + k) & 63], k % 8 == 7 ? "\n" : "");
                std::fprintf(stderr, "[fault] halted=%d SR=$%04X PC=$%08X clock=%lld ipl=%d\n",
                             cpu.isHalted(), cpu.getSR(), cpu.getPC(),
                             (long long)cpu.getClock(), currentIpl);
                return 4;
            }
        }

        // 掃描線推進
        const int budget = bus.video().h320() ? 728 : 684;
        if (lineCycles < budget) continue;
        lineCycles -= budget;

        bus.video().clearRaster();  // raster IRQ 為單線脈衝
        bus.video().clearVblankPulse();  // vblank IRQ 亦為脈衝（MAME HOLD_LINE：ack 後自動解除）
        soundIrq6 = false;          // mailbox IRQ6 同樣以脈衝處理
        vpos = (vpos + 1) % 262;
        bus.video().setScanline(vpos);

        if (vpos == 240) {
            bus.video().triggerVblank();
            bus.video().renderFrame();
            soundCpu.pulseNmi();    // MAME：vblank 時 pulse 65C02 NMI
            if (std::getenv("ACAN_DEBUG"))
                std::fprintf(stderr, "[dbg] frame=%llu pc=$%08X 65c02pc=$%04X\n",
                             (unsigned long long)bus.video().frameNumber(), cpu.getPC(), soundCpu.getPC());

            if (frameLimit > 0 && int64_t(bus.video().frameNumber()) >= frameLimit) quit = true;

#ifndef NO_SDL
            if (!headless) {
                SDL_UpdateTexture(texture, nullptr, bus.video().framebuffer().data(), UM6618::WIDTH * 4);
                SDL_RenderClear(renderer);
                SDL_RenderCopy(renderer, texture, nullptr, nullptr);
                SDL_RenderPresent(renderer);
                SDL_Event ev;
                while (SDL_PollEvent(&ev)) if (ev.type == SDL_QUIT) quit = true;
            }
#endif
        } else if (vpos < 240 && (bus.video().irqMask() & 0x10)) {
            bus.video().triggerRaster();
        }
        bus.video().tickLineIrq();   // IRQ5 line on/off 目標線判定
        applyIrq();
    }

    std::printf("[done] 幀數=%llu 最終 PC=$%08X video_flags=$%04X irq_mask=$%02X 65C02=%s boot_ack($0300)=$%02X\n",
                (unsigned long long)bus.video().frameNumber(), cpu.getPC(),
                bus.video().readReg(0x04), bus.video().irqMask(),
                soundCpu.halted() ? "HALT" : "run", bus.soundRamData()[0x300]);
    std::printf("[done] IRQ ack 計數：VBL7=%ld MBOX6=%ld LINE5=%ld RASTER4=%ld\n",
                irqAckCount[7], irqAckCount[6], irqAckCount[5], irqAckCount[4]);

    if (!screenshotPath.empty()) {
        if (writeBmp(screenshotPath, bus.video().framebuffer(), UM6618::WIDTH, UM6618::HEIGHT))
            std::printf("[done] 截圖已輸出：%s\n", screenshotPath.c_str());
        else
            std::fprintf(stderr, "錯誤：截圖寫入失敗 %s\n", screenshotPath.c_str());
    }

    if (const char *dump = std::getenv("ACAN_DUMP")) {  // 除錯：dump VRAM/palette/regs
        std::string p(dump);
        std::ofstream vf(p + ".vram", std::ios::binary);
        for (uint32_t i = 0; i < 0x10000; i++) { uint16_t w = bus.video().vramWordRaw(i); vf.put(char(w >> 8)); vf.put(char(w)); }
        std::ofstream pf(p + ".pal", std::ios::binary);
        for (int i = 0; i < 256; i++) { uint16_t w = bus.video().paletteRaw(i); pf.put(char(w >> 8)); pf.put(char(w)); }
        std::ofstream rf(p + ".regs.txt");
        for (int i = 0; i < 256; i++) rf << std::hex << i * 2 << ": " << bus.video().rawReg(i) << "\n";
        std::ofstream wf(p + ".wram", std::ios::binary);
        for (uint32_t a = 0xFC0000; a < 0xFD0000; a++) wf.put(char(bus.read8(a)));
        std::ofstream sf(p + ".sram65", std::ios::binary);
        sf.write(reinterpret_cast<char *>(bus.soundRamData()), 65536);
        std::printf("[done] dump 已輸出：%s.{vram,pal,regs.txt,wram,sram65}\n", dump);
    }

#ifndef NO_SDL
    if (!headless) {
        SDL_DestroyTexture(texture);
        SDL_DestroyRenderer(renderer);
        SDL_DestroyWindow(window);
    }
    SDL_Quit();
#endif
    return 0;
}
