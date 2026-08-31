# 音訊呈現層

UMC6619 核心固定依 3,579,545 Hz 時鐘每 80 cycle 產生一個原生樣本，名義取樣率
為 44,744.3125 Hz。平台音訊裝置不會改變這個時序；`presentation.StereoResampler`
只把已完成的核心樣本轉為 48,000 Hz signed 16-bit stereo PCM。

重取樣器以整數時戳比較輸入與輸出位置，使用相鄰原生樣本做線性插值。它不查詢
主機 wall-clock、不丟回資料到 machine，也不依賴 Ebitengine、SDL 或 cgo。headless
可用 `--wav output.wav` 將相同 PCM 寫成標準 44-byte RIFF/WAVE；沒有指定 `--wav`
時不配置累積 PCM buffer。

## 真實輸入驗證

固定 Boom Zoo ROM SHA-256
`090827d00ef8047d2c78cc173d258565b1c3ab01f0d97dc3ed8e08833d370077`，搭配完整
68k／兩塊 65C02 BIOS 與 UMC6650 key 執行 120 幀，得到：

- 原生樣本 82,158；48 kHz WAV frames 88,135。
- WAV：2 channels、16-bit、352,584 bytes。
- 非零 int16 值 139,712；peak 6,375；RMS 3,069.272。
- 完整 WAV SHA-256：
  `d470a00b9063faf0a15a3f9bc5306d506780102323387421cf96bd1a61e84942`。
- 相同輸入重跑兩次，完整 WAV 雜湊相同。

這證明格式、非靜音與決定性，不等同人耳音高、節奏、左右聲道或類比增益驗收。
