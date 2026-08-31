// Deprecated C++ oracle；production 改由純 Go 獨立實作。
// superacan-emu runner（里程碑 3+4：UM6619 音效合成 + 手把輸入）
//
// 用法：
//   superacan-emu --bios <dir> --rom <file> [--trace N] [--instructions N]
//                 [--headless] [--frames N] [--screenshot <file.bmp>]
//                 [--wav <file.wav>] [--press <frame:BTN+BTN,...>]
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
#include "state.hpp"

#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <cmath>
#include <deque>
#include <fstream>
#include <mutex>
#include <string>
#include <vector>

extern uint32_t g_dbgPc;  // bus.cpp 的除錯 watchpoint 用
extern uint32_t g_dbgA0;
extern uint32_t g_dbgA1;
extern uint32_t g_dbgSp;  // bus.cpp：A7
extern uint64_t g_dbgFrame;  // bus.cpp；ACAN_TRACE65 日誌幀號

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
        "  --screenshot <f>    結束時把最後一幀寫成 24-bit BMP\n"
        "  --wav <f>           把全程音訊寫成 48 kHz 16-bit stereo WAV\n"
        "  --press <spec>      headless 按鍵注入（P1）：frame:BTN+BTN,...（按住 10 幀）\n"
        "  --press2 <spec>     同上，注入 P2\n"
        "  --save-state <f>    結束時把模擬器全狀態寫入 <f>\n"
        "  --load-state <f>    啟動時從 <f> 載入全狀態（跳過 IPL）\n"
        "                      BTN = A/B/X/Y/L/R/START/SELECT/UP/DOWN/LEFT/RIGHT\n",
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

// ---- 音訊管線：UM6619 native 44744.3125 Hz → 48000 Hz 線性插值 ----
constexpr int AUDIO_RATE = 48000;

struct AudioPipeline {
    static constexpr double STEP = UM6619::NATIVE_RATE / AUDIO_RATE;  // native/output
    double nextT = 1.0;                  // 下一個輸出樣本的 native 時間
    int64_t idx = 0;                     // 已收到的 native 樣本數
    int16_t ringL[4096]{}, ringR[4096]{};
    std::deque<int16_t> queue;           // → SDL（interleaved S16）
    std::vector<int16_t> wav;            // headless WAV 收集（interleaved S16）
    std::mutex mtx;
    bool toSdl = false, keepWav = false;

    void push(int16_t l, int16_t r) {    // 由 UM6619 onSample 呼叫（native rate）
        ringL[idx & 4095] = l; ringR[idx & 4095] = r; idx++;
        while (nextT + 1.0 <= double(idx - 1)) {
            const int64_t i0 = int64_t(nextT);
            const double f = nextT - double(i0);
            const int16_t ol = int16_t(ringL[i0 & 4095] + std::lround((ringL[(i0 + 1) & 4095] - ringL[i0 & 4095]) * f));
            const int16_t orr = int16_t(ringR[i0 & 4095] + std::lround((ringR[(i0 + 1) & 4095] - ringR[i0 & 4095]) * f));
            nextT += STEP;
            if (keepWav) { wav.push_back(ol); wav.push_back(orr); }
            if (toSdl) {
                std::lock_guard<std::mutex> g(mtx);
                if (queue.size() > size_t(AUDIO_RATE)) queue.clear();  // 消費跟不上就丟（防延遲累積）
                queue.push_back(ol); queue.push_back(orr);
            }
        }
    }

    void sdlCallback(uint8_t *stream, int len) {  // SDL S16 stereo
        std::lock_guard<std::mutex> g(mtx);
        int16_t *out = reinterpret_cast<int16_t *>(stream);
        const size_t want = size_t(len) / 2, have = std::min(want, queue.size());
        for (size_t i = 0; i < have; i++) { out[i] = queue.front(); queue.pop_front(); }
        for (size_t i = have; i < want; i++) out[i] = 0;  // underflow → 靜音
    }
};

bool writeWav(const std::string &path, const std::vector<int16_t> &samples, int rate) {
    std::ofstream f(path, std::ios::binary);
    if (!f) return false;
    const uint32_t dataSize = uint32_t(samples.size() * 2);
    auto put32 = [&](uint32_t v) { f.put(char(v)); f.put(char(v >> 8)); f.put(char(v >> 16)); f.put(char(v >> 24)); };
    auto put16 = [&](uint16_t v) { f.put(char(v)); f.put(char(v >> 8)); };
    f.write("RIFF", 4); put32(36 + dataSize); f.write("WAVE", 4);
    f.write("fmt ", 4); put32(16); put16(1); put16(2); put32(uint32_t(rate));
    put32(uint32_t(rate) * 4); put16(4); put16(16);
    f.write("data", 4); put32(dataSize);
    for (int16_t s : samples) put16(uint16_t(s));
    return bool(f);
}

// ---- 手把按鍵位元（知識庫 memory-map.md §7 (a/b)，16-bit active low）----
constexpr uint16_t BTN_A = 0x8000, BTN_B = 0x4000, BTN_START = 0x2000, BTN_SELECT = 0x1000;
constexpr uint16_t BTN_UP = 0x0800, BTN_DOWN = 0x0400, BTN_LEFT = 0x0200, BTN_RIGHT = 0x0100;
constexpr uint16_t BTN_X = 0x0080, BTN_Y = 0x0040, BTN_L = 0x0020, BTN_R = 0x0010;

uint16_t buttonBits(const std::string &name) {  // 單鍵名稱 → 位元；不認識回 0
    if (name == "A") return BTN_A;
    if (name == "B") return BTN_B;
    if (name == "X") return BTN_X;
    if (name == "Y") return BTN_Y;
    if (name == "L") return BTN_L;
    if (name == "R") return BTN_R;
    if (name == "START") return BTN_START;
    if (name == "SELECT") return BTN_SELECT;
    if (name == "UP") return BTN_UP;
    if (name == "DOWN") return BTN_DOWN;
    if (name == "LEFT") return BTN_LEFT;
    if (name == "RIGHT") return BTN_RIGHT;
    return 0;
}

// --press 事件：frame:BTN+BTN（逗號分隔），按住 10 幀後放開
struct PressEvent { long frame; uint16_t bits; };

std::vector<PressEvent> parsePress(const std::string &spec) {
    std::vector<PressEvent> out;
    size_t pos = 0;
    while (pos <= spec.size()) {
        const size_t comma = spec.find(',', pos);
        const std::string tok = spec.substr(pos, comma == std::string::npos ? comma : comma - pos);
        const size_t colon = tok.find(':');
        if (colon != std::string::npos) {
            PressEvent ev{std::atol(tok.substr(0, colon).c_str()), 0};
            size_t b = colon + 1;
            while (b <= tok.size()) {
                const size_t plus = tok.find('+', b);
                const std::string name = tok.substr(b, plus == std::string::npos ? plus : plus - b);
                ev.bits |= buttonBits(name);
                if (plus == std::string::npos) break;
                b = plus + 1;
            }
            if (ev.bits) out.push_back(ev);
            else std::fprintf(stderr, "警告：--press 事件 '%s' 無有效按鍵，忽略\n", tok.c_str());
        }
        if (comma == std::string::npos) break;
        pos = comma + 1;
    }
    return out;
}

} // namespace

int main(int argc, char **argv) {
    std::string biosDir, romPath, screenshotPath, wavPath, pressSpec, press2Spec;
    std::string saveStatePath, loadStatePath;
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
        else if (a == "--wav") wavPath = next("--wav");
        else if (a == "--press") pressSpec = next("--press");
        else if (a == "--press2") press2Spec = next("--press2");
        else if (a == "--save-state") saveStatePath = next("--save-state");
        else if (a == "--load-state") loadStatePath = next("--load-state");
        else { usage(argv[0]); return 1; }
    }
    if (biosDir.empty() || romPath.empty()) { usage(argv[0]); return 1; }

#ifndef NO_SDL
    if (!headless && SDL_Init(SDL_INIT_VIDEO | SDL_INIT_AUDIO) != 0) {
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
    bus.onSoundIoWrite = [&soundCpu](uint16_t a, uint8_t v) { soundCpu.writeFrom68k(a, v); };

    // UM6618 sprite DMA 需要回寫整個位址空間
    bus.video().busRead16 = [&bus](uint32_t a) { return bus.read16(a); };
    bus.video().busWrite16 = [&bus](uint32_t a, uint16_t v) { bus.write16(a, v); };

    Cpu68k cpu(bus);

    // 音訊管線（UM6619 → 重取樣 → SDL / WAV）
    AudioPipeline audio;
    audio.keepWav = !wavPath.empty();
    soundCpu.soundChip().onSample = [&audio](int16_t l, int16_t r) { audio.push(l, r); };

    // 手把輸入（P1+P2；16-bit active low，memory-map.md §7）
    uint16_t padState[2] = { 0xFFFF, 0xFFFF };
    bus.setPad(0, padState[0]);
    bus.setPad(1, padState[1]);
    std::vector<PressEvent> pressEvents = parsePress(pressSpec);
    for (const auto &ev : pressEvents)
        std::printf("[input] 預約按鍵（P1）：frame %ld press $%04X（10 幀）\n", ev.frame, ev.bits);
    std::vector<PressEvent> pressEvents2 = parsePress(press2Spec);
    for (const auto &ev : pressEvents2)
        std::printf("[input] 預約按鍵（P2）：frame %ld press $%04X（10 幀）\n", ev.frame, ev.bits);

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

    // ---- save state（格式見 state.hpp）----
    // runner 狀態變數提前宣告（主迴圈直接使用這些）
    uint64_t romHash = bus.romHash();
    int vpos = 0;
    int64_t lineCycles = 0;
    int soundCycleAcc = 0;   // 65C02 = 68k/3（§1 (a)）
    int currentIpl = 0;
    bool frcPending = false;
    int64_t frcNext = -1;
    auto collectState = [&]() -> std::vector<uint8_t> {
        StateWriter w;
        bus.saveState(w);
        soundCpu.saveState(w);
        cpu.saveState(w);
        w.put(vpos); w.put(lineCycles); w.put(soundCycleAcc);
        w.put(currentIpl); w.put(soundIrq6); w.put(frcPending); w.put(frcNext);
        return w.buf;
    };
    auto applyState = [&](const std::vector<uint8_t> &buf) -> bool {
        StateReader r(buf.data(), buf.size());
        bus.loadState(r);
        soundCpu.loadState(r);
        cpu.loadState(r);
        r.get(vpos); r.get(lineCycles); r.get(soundCycleAcc);
        r.get(currentIpl); r.get(soundIrq6); r.get(frcPending); r.get(frcNext);
        return r.ok && r.left == 0;
    };

    const uint32_t expectedEntry = bus.cartVector(1);
    std::printf("[info] 卡帶向量表入口 PC=$%08X SSP=$%08X\n", expectedEntry, bus.cartVector(0));

#ifndef NO_SDL
    SDL_Window *window = nullptr;
    SDL_Renderer *renderer = nullptr;
    SDL_Texture *texture = nullptr;
    SDL_AudioDeviceID audioDev = 0;
    if (!headless) {
        window = SDL_CreateWindow("superacan-emu", SDL_WINDOWPOS_CENTERED, SDL_WINDOWPOS_CENTERED,
                                  UM6618::WIDTH * 3, UM6618::HEIGHT * 3, SDL_WINDOW_RESIZABLE);
        renderer = window ? SDL_CreateRenderer(window, -1, SDL_RENDERER_ACCELERATED | SDL_RENDERER_PRESENTVSYNC) : nullptr;
        if (!renderer && window)  // 無 accelerated（如 dummy driver）時退回 software
            renderer = SDL_CreateRenderer(window, -1, SDL_RENDERER_SOFTWARE);
        texture = renderer ? SDL_CreateTexture(renderer, SDL_PIXELFORMAT_ARGB8888,
                                               SDL_TEXTUREACCESS_STREAMING, UM6618::WIDTH, UM6618::HEIGHT) : nullptr;
        if (!texture) {
            std::fprintf(stderr, "警告：SDL 視窗建立失敗（%s），改為 headless\n", SDL_GetError());
            headless = true;
        } else {
            SDL_AudioSpec want{};
            want.freq = AUDIO_RATE;
            want.format = AUDIO_S16SYS;
            want.channels = 2;
            want.samples = 1024;
            want.callback = [](void *ud, Uint8 *stream, int len) {
                static_cast<AudioPipeline *>(ud)->sdlCallback(stream, len);
            };
            want.userdata = &audio;
            audioDev = SDL_OpenAudioDevice(nullptr, 0, &want, nullptr, 0);
            if (audioDev) {
                audio.toSdl = true;
                SDL_PauseAudioDevice(audioDev, 0);
                std::printf("[audio] SDL2 音訊裝置已開啟（%d Hz stereo）\n", AUDIO_RATE);
            } else {
                std::fprintf(stderr, "警告：SDL 音訊開啟失敗（%s），無聲繼續\n", SDL_GetError());
            }
        }
    }
#endif

    // ---- 主迴圈：IPL 階段 → 幀時序階段 ----
    bool atCartEntry = false, handoverLogged = false;
    bool lockoutLogged = false, licenseLogged = false;
    long traced = 0, postRun = 0;
    const long maxIplInstrs = 2000000;
    long total = 0;
    bool quit = false;

    // save state 熱鍵用槽位檔名
    int stateSlot = 0;
    auto slotPath = [&]() -> std::string {
        return romPath + ".st" + std::to_string(stateSlot);
    };

    // --load-state：跳過 IPL，直接進入卡帶後狀態
    if (!loadStatePath.empty()) {
        std::vector<uint8_t> buf;
        bool mism = false;
        if (!readStateFile(loadStatePath.c_str(), romHash, buf, &mism) || !applyState(buf)) {
            std::fprintf(stderr, "錯誤：save state 載入失敗：%s\n", loadStatePath.c_str());
            return 1;
        }
        if (mism) std::fprintf(stderr, "警告：save state 的 ROM hash 與目前 ROM 不符（仍載入）\n");
        atCartEntry = true;
        if (frameLimit > 0) frameLimit += long(bus.video().frameNumber());  // --frames = 再多跑 N 幀
        std::printf("[state] 已載入 %s（frame=%llu）\n", loadStatePath.c_str(),
                    (unsigned long long)bus.video().frameNumber());
    }

    char dasm[128];
    // FRC（$E90014/$16 → 68k IRQ3；MAME update_frc_state 的 case 表 (b)，
    // 其本身即 case-by-case HACK，真實公式待查證）
    auto frcUpdate = [&]() {
        const uint16_t ctrl = bus.frcControl();
        if ((ctrl & 0xFF00) != 0xA200) { frcNext = -1; return; }
        const uint32_t period = (uint32_t(ctrl & 0xFF) << 16) | bus.frcFreq();
        int64_t cyc = -1;
        switch (ctrl & 0xF) {
        case 0: cyc = 10738635; break;            // MAME HACK：1 Hz
        case 1: cyc = 1024LL * period; break;
        case 0xF: cyc = 8192LL * period; break;
        default: break;                            // 未知組合：關閉（MAME popmessage）
        }
        frcNext = (cyc < 0) ? -1 : cpu.getClock() + cyc;
    };
    bus.onFrcWrite = frcUpdate;
    auto applyIrq = [&]() {
        // vblank（IRQ7）> mailbox（IRQ6）> raster（IRQ4）；mask 在 $E90010（§6 (b)）
        int lvl = 0;
        if (bus.video().vblankPending()) lvl = 7;
        else if (soundIrq6) lvl = 6;
        else if (bus.video().irq5Pending()) lvl = 5;
        else if (bus.video().rasterActive()) lvl = 4;
        else if (frcPending) lvl = 3;
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
        else if (level == 3) { frcPending = false; frcUpdate(); }  // 重新排程（MAME 行為）
        applyIrq();
    };

    uint32_t previousProbePc = 0xFFFFFFFF;
    while (!quit) {
        const uint32_t pc = cpu.getPC();
        g_dbgPc = pc;
        g_dbgSp = cpu.getA(7);
        g_dbgA0 = cpu.getA(0);
        g_dbgA1 = cpu.getA(1);

        // F003 runtime decompressor probe.  The cartridge copies ROM $7399E
        // to Work RAM $FFFF8000; log only its entry and final RTS so the
        // source/destination contract can be reconstructed without changing
        // execution or turning this deprecated oracle into a debugger API.
        if (std::getenv("ACAN_WATCH") && pc != previousProbePc &&
            (pc == 0xFFFF8000 || pc == 0xFFFF80E2)) {
            std::fprintf(stderr,
                         "[watchdecomp] f=%llu pc=$%08X "
                         "a0=$%08X a1=$%08X a2=$%08X a3=$%08X "
                         "d0=$%08X d1=$%08X d2=$%08X d3=$%08X\n",
                         (unsigned long long)g_dbgFrame, pc,
                         cpu.getA(0), cpu.getA(1), cpu.getA(2), cpu.getA(3),
                         cpu.getD(0), cpu.getD(1), cpu.getD(2), cpu.getD(3));
        }
        previousProbePc = pc;

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

        // FRC 計時器（IRQ3，HOLD_LINE）
        if (frcNext > 0 && cpu.getClock() >= frcNext) { frcPending = true; frcNext = -1; applyIrq(); }

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
            g_dbgFrame = bus.video().frameNumber();
            soundCpu.pulseNmi();    // MAME：vblank 時 pulse 65C02 NMI
            if (std::getenv("ACAN_STAGING")) {   // 臨時：追蹤 staging buffer 前 16 word 變化
                static uint16_t prev[16] = {};
                uint16_t cur[16];
                for (int i = 0; i < 16; i++) cur[i] = bus.read16(0xFC0800 + i * 2);
                if (std::memcmp(prev, cur, sizeof(cur)) != 0) {
                    std::fprintf(stderr, "[staging] f=%llu", (unsigned long long)bus.video().frameNumber());
                    for (int i = 0; i < 16; i++) std::fprintf(stderr, " %04x", cur[i]);
                    std::fprintf(stderr, "\n");
                    std::memcpy(prev, cur, sizeof(cur));
                }
            }
            if (std::getenv("ACAN_DEBUG"))
                std::fprintf(stderr, "[dbg] frame=%llu pc=$%08X 65c02pc=$%04X\n",
                             (unsigned long long)bus.video().frameNumber(), cpu.getPC(), soundCpu.getPC());

            if (frameLimit > 0 && int64_t(bus.video().frameNumber()) >= frameLimit) quit = true;

            // --press/--press2 注入：到幀按下、10 幀後放開
            if (!pressEvents.empty() || !pressEvents2.empty()) {
                const long f = long(bus.video().frameNumber());
                for (const auto &ev : pressEvents) {
                    if (f == ev.frame) padState[0] &= ~ev.bits;
                    if (f == ev.frame + 10) padState[0] |= ev.bits;
                }
                for (const auto &ev : pressEvents2) {
                    if (f == ev.frame) padState[1] &= ~ev.bits;
                    if (f == ev.frame + 10) padState[1] |= ev.bits;
                }
                bus.setPad(0, padState[0]);
                bus.setPad(1, padState[1]);
            }

#ifndef NO_SDL
            if (!headless) {
                SDL_UpdateTexture(texture, nullptr, bus.video().framebuffer().data(), UM6618::WIDTH * 4);
                SDL_RenderClear(renderer);
                SDL_RenderCopy(renderer, texture, nullptr, nullptr);
                SDL_RenderPresent(renderer);
                SDL_Event ev;
                while (SDL_PollEvent(&ev)) {
                    if (ev.type == SDL_QUIT) { quit = true; continue; }
                    if (ev.type == SDL_KEYDOWN || ev.type == SDL_KEYUP) {
                        if (ev.key.repeat) continue;
                        const bool down = ev.type == SDL_KEYDOWN;
                        if (down && ev.key.keysym.sym == SDLK_F5) {   // 存檔
                            if (writeStateFile(slotPath().c_str(), romHash, collectState()))
                                std::printf("[state] 已存檔：%s\n", slotPath().c_str());
                            else std::fprintf(stderr, "錯誤：無法寫入 %s\n", slotPath().c_str());
                            continue;
                        }
                        if (down && ev.key.keysym.sym == SDLK_F6) {   // 切槽
                            stateSlot = (stateSlot + 1) % 10;
                            std::printf("[state] 槽位 → %d\n", stateSlot);
                            continue;
                        }
                        if (down && ev.key.keysym.sym == SDLK_F7) {   // 讀檔
                            std::vector<uint8_t> buf;
                            bool mism = false;
                            if (readStateFile(slotPath().c_str(), romHash, buf, &mism) && applyState(buf)) {
                                if (mism) std::fprintf(stderr, "警告：ROM hash 不符（仍載入）\n");
                                std::printf("[state] 已讀檔：%s\n", slotPath().c_str());
                            } else {
                                std::fprintf(stderr, "錯誤：讀檔失敗 %s\n", slotPath().c_str());
                            }
                            continue;
                        }
                        uint16_t bit = 0;
                        int player = 0;
                        // P1：方向鍵 + Z/X/A/S/Q/W = A/B/X/Y/L/R（Bcan.ini 預設風格），
                        // Enter=Start、右 Shift=Select
                        // P2：I/J/K/L 方向 + U/O/N/M = A/B/X/Y、逗號/句號=L/R、
                        // 右 Ctrl=Start、左 Shift=Select
                        switch (ev.key.keysym.sym) {
                        case SDLK_UP: bit = BTN_UP; break;
                        case SDLK_DOWN: bit = BTN_DOWN; break;
                        case SDLK_LEFT: bit = BTN_LEFT; break;
                        case SDLK_RIGHT: bit = BTN_RIGHT; break;
                        case SDLK_z: bit = BTN_A; break;
                        case SDLK_x: bit = BTN_B; break;
                        case SDLK_a: bit = BTN_X; break;
                        case SDLK_s: bit = BTN_Y; break;
                        case SDLK_q: bit = BTN_L; break;
                        case SDLK_w: bit = BTN_R; break;
                        case SDLK_RETURN: bit = BTN_START; break;
                        case SDLK_RSHIFT: bit = BTN_SELECT; break;
                        case SDLK_i: player = 1; bit = BTN_UP; break;
                        case SDLK_k: player = 1; bit = BTN_DOWN; break;
                        case SDLK_j: player = 1; bit = BTN_LEFT; break;
                        case SDLK_l: player = 1; bit = BTN_RIGHT; break;
                        case SDLK_u: player = 1; bit = BTN_A; break;
                        case SDLK_o: player = 1; bit = BTN_B; break;
                        case SDLK_n: player = 1; bit = BTN_X; break;
                        case SDLK_m: player = 1; bit = BTN_Y; break;
                        case SDLK_COMMA: player = 1; bit = BTN_L; break;
                        case SDLK_PERIOD: player = 1; bit = BTN_R; break;
                        case SDLK_RCTRL: player = 1; bit = BTN_START; break;
                        case SDLK_LSHIFT: player = 1; bit = BTN_SELECT; break;
                        default: break;
                        }
                        if (bit) {
                            if (down) padState[player] &= ~bit; else padState[player] |= bit;
                            bus.setPad(player, padState[player]);
                        }
                    }
                }
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
    std::printf("[done] IRQ ack 計數：VBL7=%ld MBOX6=%ld LINE5=%ld RASTER4=%ld FRC3=%ld\n",
                irqAckCount[7], irqAckCount[6], irqAckCount[5], irqAckCount[4], irqAckCount[3]);

    if (!screenshotPath.empty()) {
        if (writeBmp(screenshotPath, bus.video().framebuffer(), UM6618::WIDTH, UM6618::HEIGHT))
            std::printf("[done] 截圖已輸出：%s\n", screenshotPath.c_str());
        else
            std::fprintf(stderr, "錯誤：截圖寫入失敗 %s\n", screenshotPath.c_str());
    }

    if (!saveStatePath.empty()) {
        if (writeStateFile(saveStatePath.c_str(), romHash, collectState()))
            std::printf("[done] save state 已輸出：%s\n", saveStatePath.c_str());
        else
            std::fprintf(stderr, "錯誤：save state 寫入失敗 %s\n", saveStatePath.c_str());
    }

    if (!wavPath.empty()) {
        if (writeWav(wavPath, audio.wav, AUDIO_RATE))
            std::printf("[done] 音訊已輸出：%s（%zu samples，%.1f 秒，UM6619 active=$%04X）\n",
                        wavPath.c_str(), audio.wav.size() / 2,
                        audio.wav.size() / 2 / double(AUDIO_RATE), soundCpu.soundChip().activeChannels());
        else
            std::fprintf(stderr, "錯誤：WAV 寫入失敗 %s\n", wavPath.c_str());
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
    if (audioDev) SDL_CloseAudioDevice(audioDev);
    if (!headless) {
        SDL_DestroyTexture(texture);
        SDL_DestroyRenderer(renderer);
        SDL_DestroyWindow(window);
    }
    SDL_Quit();
#endif
    return 0;
}
