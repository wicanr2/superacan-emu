# 介面畫面驗證

更新日期：2026-09-01。

介面的迴歸靠兩種雜湊：**畫面雜湊**守住版面不被意外改動，**卡帶基準**守住介面沒有
滲進模擬路徑。兩者都由 `go test ./ui/` 與 `go test ./machine/` 自動比對，
數值同時記錄在這裡供人查閱。

## 畫面雜湊

覆蓋層畫在表面的原生解析度，所以雜湊會隨表面大小改變。固定兩組表面，
其餘尺寸不入驗證：

| 設定檔 | 表面 | Scale |
|---|---|---:|
| `compact` | 960×720 | 1 |
| `touch` | 1280×720 | 1 |

取像方式：先把固定的測試 framebuffer 放大鋪滿整個表面，再畫覆蓋層，
對整張 RGBA 的 `Pix` 取 SHA-256。測試資料是固定的漸層與固定的存檔槽現況
（槽 0、3 可讀，槽 7 被拒絕，其餘為空），所以雜湊只受介面本身影響。

| 畫面 | 表面 | SHA-256 |
|---|---|---|
| S3 覆蓋選單 | 960×720 `compact` | `ff406b886d278a0c70600e20d8b53161713df90cdf58434635ab5ef9546e5ef0` |
| S3 覆蓋選單 | 1280×720 `touch` | `46eef64f92a9890101d108c61e033ab431b5b08fd9570c3a798bdfc98a84a924` |
| S3 經 Down×3、Confirm、Cancel | 960×720 `compact` | `73ad19293711c6a1b51c6d86f6f20f63d68fdcf8592f990d8c09ecb14ce243ca` |
| S4 存檔槽（存檔模式） | 960×720 `compact` | `8279ec7eade67dca4b70e3cca03deb3f0bd9cd8573a170119cb0a06f52d007cc` |
| S4 存檔槽（存檔模式） | 1280×720 `touch` | `ce19280911c7abca897d29aacabcd241ac646e01f5a713029a45721b6b8273e0` |
| D1 覆寫確認 | 960×720 `compact` | `e1bf95ba55c979941a854d0e07d8a964b630a26e51bf28c5e81b99a6eb4c72e6` |

雜湊只能守住「沒有意外變動」，看不出版面本來就畫錯。要用人眼檢查時把畫面另存
PNG：

```sh
ACAN_UI_DUMP=/src/build/uidump docker/go.sh test ./ui/ -count=1
```

版面刻意改動時，先看 PNG 確認新版面正確，再把新雜湊填回 `ui/render_test.go`
與這張表——順序反過來就等於用雜湊掩蓋錯誤。

## 卡帶基準（C10）

介面不得滲進模擬路徑。每一個介面階段完成後，九款卡帶的 1200-frame 執行結果
必須與下表相同。

固定輸入與 3600-frame 結果見 [`verify-rom-matrix.md`](verify-rom-matrix.md)。

| ROM | 68000 指令 | 非黑像素 | framebuffer SHA-256 |
|---|---:|---:|---|
| Boom Zoo | 17,369,003 | 22,533 | `3784f8663b1c3a869498d2e14c0b948c598d50d15cf54b6f5380c9b294155562` |
| Formosa Duel | 19,270,779 | 76,800 | `0856269e7b402158e953de03d0553128d720ef64f29afc97403f93471404d587` |
| Journey to the Laugh | 17,778,132 | 7,645 | `42285d489bd74a5c5fd0d66700ed7e7c8b2b83f4855612d7dec4db07c30b146e` |
| Monopoly – Adventure in Africa | 11,827,355 | 76,800 | `c254c50d5f85dd6ede60b82c8b2a07ca2ca8ccd41e9bfbe65a1c45299083d582` |
| Sango Fighter | 11,634,924 | 50,390 | `412213dac64ec07ef8db6ee69f4a90a351880f11c3229b378647d05f559bd505` |
| Speedy Dragon | 18,513,698 | 33,125 | `d3e5336af35b4c5bdac93dca6e1f3686be861564f16d69a97ef8fa947a5b7d67` |
| Super Taiwanese Baseball League | 17,572,195 | 45,080 | `e28f1c411a389ecd46206d8006e1e9b54f62a75047bcb2e64b7f12763f094023` |
| The Son of Evil | 16,727,440 | 32,274 | `bbd3a45fb5d27acf8e6caef06f5c9f7d00f8743d2ad42b0bbb8baea2d23bca73` |
| Super Dragon Force（雙部分 ZIP） | 23,988,611 | 5,066 | `b9bba41025f239508306cd4a8e6d58c632681e05b9e24c997f8695bb40e8eedf` |

重跑方式（ROM 與 BIOS 唯讀掛載，不入版控）：

```sh
export ACAN_MEDIA_DIR=…/Bcan008b/ROMS ACAN_BIOS_DIR=…/bios
docker/go.sh run ./cmd/acan-headless --ipl /bios/internal_68k.bin --key /bios/umc6650.bin \
    --sound-bios1 /bios/internal_6502_1.bin --sound-bios2 /bios/internal_6502_2.bin \
    --rom "/media/Boom Zoo (Taiwan).bin" --frames 1200
```
