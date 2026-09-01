#!/bin/bash
# 在 superacan-bcan-wine 容器內以 Wine 執行 Bcan 0.0.8b，並定時觸發它自己的
# 截圖熱鍵（F8），取得 UM6618 顯示孔徑的 320x240 PNG 序列當畫面 oracle。
#
# 容器內路徑約定：
#   /work/bcan        可寫的 Bcan 工作目錄（Bcan.exe、Bcan.ini、bios/、ROMS/、snap/）
#   /work/wineprefix  WINEPREFIX
#
# 用法：bcan-oracle.sh <ROM 檔名> <截圖張數> <每張間隔秒數> <輸出前綴>
#
# 已知環境限制（實測，非推測）：
# - Xvfb 下沒有視窗管理員時 Wine 收不到 xdotool 的鍵盤事件，必須先跑 openbox。
# - Ctrl+O 快捷鍵在此環境無效，開檔要用滑鼠點「檔案(F)」→「開啟 ROM(O)...」。
# - Bcan 沒有以 argv 載入 ROM 的路徑；只能走檔案對話框。
set -eu

ROM="$1"; SHOTS="$2"; INTERVAL="$3"; OUT="$4"
export PATH=/usr/lib/wine:$PATH
export WINEDEBUG="${WINEDEBUG:--all}"
DISPLAY_NUM="${DISPLAY_NUM:-77}"

Xvfb ":${DISPLAY_NUM}" -screen 0 1280x960x24 >"/work/${OUT}.xvfb.log" 2>&1 &
sleep 2
export DISPLAY=":${DISPLAY_NUM}"
openbox >"/work/${OUT}.openbox.log" 2>&1 &
sleep 2

cd /work/bcan
wine64 Bcan.exe >"/work/${OUT}.wine.log" 2>&1 &
sleep 20

WID=$(xdotool search --name "^Bcan" | head -1)
xdotool windowactivate --sync "$WID"
sleep 1
xdotool mousemove 24 40 click 1        # 檔案(F)
sleep 2
xdotool mousemove 70 60 click 1        # 開啟 ROM(O)...
sleep 4
xdotool type --delay 40 "Z:\\work\\bcan\\ROMS\\${ROM}"
sleep 1
xdotool key Return

for index in $(seq 1 "$SHOTS"); do
    sleep "$INTERVAL"
    WID=$(xdotool search --name "^Bcan" | head -1)
    xdotool windowactivate --sync "$WID"
    xdotool key F8
    sleep 1
    printf 'shot %s at t=%s s\n' "$index" "$((index * INTERVAL))"
done

sleep 2
ls -la /work/bcan/snap > "/work/${OUT}.snap.txt" 2>&1
import -window root "/work/${OUT}.root.png" || true
/usr/lib/wine/wineserver -k 2>/dev/null || true
sleep 1
