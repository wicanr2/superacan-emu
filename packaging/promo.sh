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
# 存檔存在**遊玩中**而不是標題畫面。這些遊戲從開機到標題要數千幀，標題到可操作
# 又要按兩次確認鍵並等轉場——影片沒有那個預算。預建時把這一段跑完再存，正式錄影
# 讀檔就直接落在有東西看的畫面上。
#
# 確認鍵是 **A，而且要按住**（`a*30`）：預設的十幀在這些遊戲的標題選單上不夠長，
# START 完全沒有作用。這是量出來的，不是從按鍵名稱推的。
# 另一個坑是**讀檔／載入之後不能馬上按**：載入當下畫面還在淡入，那時的按鍵會被
# 吃掉。實測要隔約 400 tick。
#
# 每款各自檢查，不是「有任何一個存檔就整段跳過」——只重建其中一款時會需要。
have_state() { [ -f "$STATES/$1/slot0.acanstate" ]; }

# Boom Zoo：標題在 6000 幀，A 選單、A 選角色，1-1 開場之後就是可操作畫面。
if have_state "Boom Zoo (Taiwan).bin"; then
  echo "=== 已有 Boom Zoo 存檔"
else
  echo "=== 預建存檔：Boom Zoo（遊玩中）"
  run --max-ticks 8100 \
      --ui-script "60:down,90:confirm,120:confirm,8001:hksave_state" \
      --press "6600:a*30,7000:a*30" >/dev/null
fi

# Monopoly：只當影片結尾的第二片卡帶，停在標題畫面就夠，不必進到遊戲。
if have_state "Monopoly - Adventure in Africa (Taiwan).bin"; then
  echo "=== 已有 Monopoly 存檔"
else
  echo "=== 預建存檔：Monopoly（標題 frame 3600）"
  run --max-ticks 3900 \
      --ui-script "60:down,90:confirm,120:down,150:down,180:down,210:confirm,3811:hksave_state" >/dev/null
fi

find "$STATES" -name 'slot*.acanstate' | sort

# 正式錄影。腳本事件與按鍵注入用的都是主機迴圈次數，所以兩條時間線對得上。
# 讀檔之後按 Start，再留六百多個 tick 讓遊戲把畫面換過去——那是遊戲自己的節奏，
# 不是模擬器慢。
#
# 影片的重點之一是介面，所以覆蓋層的每一個畫面都走一遍：存檔槽（存與讀兩種模式）、
# 金手指、設定底下六個子畫面、診斷，再加上啟動畫面的「關於」。每個畫面停約
# 200 tick（3.3 秒）——比讀完一頁需要的時間短，但足夠看清楚版面。
#
# 焦點是有狀態的：cancel 回上一層之後焦點停在原來那一列，所以下面每個 down 的
# 數量都是從「上一次停在哪」算出來的，不是從 0 重數。改動任何一段都要重算後面。
#
# S0 啟動畫面（--config none 沒有最近清單）：0 韌體、1 選擇卡帶、2 關於、3 離開
# S3 覆蓋選單：0 繼續、1 存檔、2 讀檔、3 重置、4 金手指、5 設定、6 診斷、
#              7 截圖、8 錄影、9 退出卡帶、10 結束
# S5 設定：0 輸入、1 熱鍵、2 畫面、3 音訊、4 語言、5 觸控
UI_SCRIPT="\
180:down,210:down,260:confirm,\
620:cancel,680:up,730:confirm,\
800:down,850:down,900:down,950:up,1000:up,1050:up,\
1120:confirm,\
1500:hkload_state,\
2100:menu,2160:down,2200:confirm,\
2450:cancel,2500:down,2540:confirm,\
2790:cancel,2840:down,2880:down,2920:confirm,\
3170:cancel,3220:down,3260:confirm,\
3320:confirm,3570:cancel,\
3620:down,3660:confirm,3890:cancel,\
3940:down,3980:confirm,4210:cancel,\
4260:down,4300:confirm,4530:cancel,\
4580:down,4620:confirm,4850:cancel,\
4900:down,4940:confirm,5170:cancel,\
5220:cancel,5270:down,5310:confirm,\
5560:cancel,5610:down,5650:down,5690:down,5740:confirm,\
5800:down,5850:confirm,\
5910:down,5950:down,5990:down,6050:confirm,\
6350:hkload_state"

# 開選單提示只在「載入卡帶之後、還沒開過選單」時出現，所以 1120 載入到 2100 開選單
# 之間那一段本來就會演到它，不必另外安排。
PRESS="1560:right*30,1640:a*15,1690:left*40,1790:down*30,\
1870:a*15,1920:up*40,2010:right*25"

# 6800 而不是整數 7000：Monopoly 的標題畫面約在 6850 tick 開始自己淡出進 attract
# mode，再往後錄就會以一片純色收尾。
TICKS=${ACAN_PROMO_TICKS:-6800}
echo "=== 錄製 $TICKS tick（約 $((TICKS / 60)) 秒）"
run --max-ticks "$TICKS" --audio-sink "cat > /dev/null" \
    --ui-script "$UI_SCRIPT" --press "$PRESS" \
    --record "$OUT/promo.avi" >/dev/null

ls -l "$OUT/promo.avi"

# 只編一份。兩份的分工來自「一份進版控、一份留全畫質」，影片改掛 Release 附件
# 之後就沒有進版控的那一份了；而下載端要的是小檔，所以留 crf 26。
# -tune animation 對這種大面積平色的畫面有效，介面文字與 crf 20 肉眼無異。
if [ -n "${ACAN_PROMO_NO_ENCODE:-}" ]; then
    echo "=== ACAN_PROMO_NO_ENCODE 有設，跳過編碼"
    exit 0
fi

echo "=== 轉成 H.264 MP4"
ffmpeg -loglevel error -y -i "$OUT/promo.avi" \
    -c:v libx264 -preset veryslow -tune animation -crf 26 -pix_fmt yuv420p \
    -c:a aac -b:a 96k -movflags +faststart \
    "$OUT/superacan-emu-promo.mp4"
ls -l "$OUT/superacan-emu-promo.mp4"
ffprobe -loglevel error -show_entries format=duration:stream=codec_name,width,height,r_frame_rate \
    -of default=noprint_wrappers=1 "$OUT/superacan-emu-promo.mp4"
