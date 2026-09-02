#!/bin/sh
# 用發行的 AppImage 錄製展示影片。在容器內執行（superacan-package image）。
#
#   packaging/promo.sh <輸出目錄>
#
# 需要掛好 /bios（主機韌體）與 /media（卡帶）。ROM 與 BIOS 不隨本 repo 散布。
#
# 影片是「用發行產物錄的」而不是「用開發中的程式錄的」：跑的就是那個 AppImage，
# 輸入是腳本，所以同一份腳本在同一份 AppImage 上會得到同一段影片。
set -eu

OUT=${1:?用法：promo.sh <輸出目錄>}
APP=${ACAN_APPIMAGE:-/build/SuperACan-x86_64.AppImage}
BIOS="--ipl /bios/internal_68k.bin --key /bios/umc6650.bin --sound-bios1 /bios/internal_6502_1.bin --sound-bios2 /bios/internal_6502_2.bin"
STATES="$OUT/states"

mkdir -p "$OUT" "$STATES"
Xvfb :99 -screen 0 1280x800x24 >/dev/null 2>&1 &
sleep 2
export DISPLAY=:99

# 影片的內容來自 AppImage 裡的執行檔，不是工作區裡的原始碼。印出雜湊，
# 錄出來的影片才回溯得到某一份執行檔。
echo "=== AppImage: $(sha256sum "$APP" | cut -c1-16)…"

run() { "$APP" $BIOS --rom-dir /media --state-root "$STATES" --config none \
        --scale 3 --pace=false --capture-dir "$OUT" "$@"; }

# 預先做存檔。走的是與正式錄影同一條「啟動畫面 → 瀏覽器 → 載入」的路，
# 存檔目錄才會落在同一個地方；直接用 --rom 開的話 StateDir 是另一個值。
#
# 每款遊戲存兩個槽：槽 0 是標題畫面，槽 1 是按下 Start 之後的可操作畫面。
# 按 Start 到畫面真的換過去要六百多幀，直接錄會變成十秒的過場；分兩個槽之後
# 影片裡兩個畫面都看得到，而且順便演到「切換存檔槽」。
# 存檔已經在的話就跳過預建：那一段要跑一萬多個 tick，重錄影片時不必再做一次。
if [ -n "$(find "$STATES" -name 'slot0.acanstate' 2>/dev/null)" ]; then
  echo "=== 已有存檔，跳過預建"
else
# 每款只用槽 0。存檔槽索引是設定檔裡的值，換卡帶不會重設，所以腳本一旦動過
# 「下一個槽」，後面每一片卡帶都會跟著偏——偏到不存在的槽就會跳出錯誤列，
# 而錯誤列會吃掉下一個確認鍵，整條腳本從那裡開始失準。不動它就沒有這個問題。
#
# 存檔的 frame 取自 docs/screenshots 已驗證過的畫面：Boom Zoo 6000、
# Monopoly 3600、Speedy Dragon 1200。載入卡帶的 tick 要加回去。
echo "=== 預建存檔：Boom Zoo（標題 frame 6000）"
run --max-ticks 6200 \
    --ui-script "60:down,90:confirm,120:confirm,6121:hksave_state" >/dev/null
echo "=== 預建存檔：Monopoly（標題 frame 3600）"
run --max-ticks 3900 \
    --ui-script "60:down,90:confirm,120:down,150:down,180:down,210:confirm,3811:hksave_state" >/dev/null
echo "=== 預建存檔：Speedy Dragon（開場風景 frame 1200）"
run --max-ticks 1550 \
    --ui-script "60:down,90:confirm,120:down,150:down,180:down,210:down,240:down,270:confirm,1471:hksave_state" >/dev/null
fi
find "$STATES" -name 'slot*.acanstate' | sort

# 正式錄影。腳本事件與按鍵注入用的都是主機迴圈次數，所以兩條時間線對得上。
# 讀檔之後按 Start，再留六百多個 tick 讓遊戲把畫面換過去——那是遊戲自己的節奏，
# 不是模擬器慢。
UI_SCRIPT="220:down,280:confirm,\
340:down,400:down,460:down,520:up,550:up,580:up,\
620:confirm,\
1000:hkload_state,\
1540:menu,1580:down,1610:down,1640:confirm,\
1890:cancel,1930:down,1960:down,1990:down,2020:down,2050:confirm,\
2330:cancel,2360:down,2390:down,2420:down,2450:confirm,\
2510:down,2570:confirm,2610:down,2640:down,2670:down,2720:confirm,\
2790:hkload_state,\
3340:menu,3370:down,3400:down,3430:down,3460:down,3490:down,3520:down,3550:down,3580:down,3610:down,3640:confirm,\
3700:down,3760:confirm,3800:down,3830:down,3860:down,3890:down,3920:down,3960:confirm,\
4030:hkload_state"

PRESS="1060:start,1200:right*25,1260:left*25,1320:down*20,\
2850:start,3000:down*20,3060:up*20,\
4100:right*40,4160:left*40"

echo "=== 錄製 4200 tick（約 70 秒）"
run --max-ticks 4200 --audio-sink "cat > /dev/null" \
    --ui-script "$UI_SCRIPT" --press "$PRESS" \
    --record "$OUT/promo.avi" >/dev/null

ls -l "$OUT/promo.avi"
echo "=== 轉成 H.264 MP4"
ffmpeg -loglevel error -y -i "$OUT/promo.avi" \
    -c:v libx264 -preset medium -crf 20 -pix_fmt yuv420p \
    -c:a aac -b:a 128k -movflags +faststart \
    "$OUT/superacan-emu-promo.mp4"
ls -l "$OUT/superacan-emu-promo.mp4"

# 進版控的那一份另外壓一次。二進位檔每重錄一次就在 git 歷程留一份完整副本，
# 所以進 repo 的版本要小；`-tune animation` 對這種大面積平色的畫面有效，
# crf 26 下介面文字與 crf 20 肉眼無異，體積約少一半。
echo "=== 轉成進版控用的 MP4"
ffmpeg -loglevel error -y -i "$OUT/promo.avi" \
    -c:v libx264 -preset veryslow -tune animation -crf 26 -pix_fmt yuv420p \
    -c:a aac -b:a 96k -movflags +faststart \
    "$OUT/superacan-emu-promo-repo.mp4"
ls -l "$OUT/superacan-emu-promo-repo.mp4"
ffprobe -loglevel error -show_entries format=duration:stream=codec_name,width,height,r_frame_rate \
    -of default=noprint_wrappers=1 "$OUT/superacan-emu-promo.mp4"
