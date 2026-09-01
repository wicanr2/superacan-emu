// Package ui 是與前端無關的介面層。
//
// 三條邊界（見 docs/ui-design.md §3）：
//
//  1. 本套件不 import 任何前端套件。Ebitengine、xgb、gomobile 都不得出現在相依
//     圖裡；TestNoFrontendDependencies 以 go list -deps 守住這條。
//  2. 本套件不持有 *machine.System。它只拿到唯讀的 Snapshot 介面，要求動作時
//     產生 Intent，由入口在 frame 邊界執行。UI 沒有寫入模擬核心的能力，這是
//     程式結構上的保證，不是靠自律。
//  3. 覆蓋層畫在表面的原生解析度，不畫在 320×240 的遊戲畫面座標系上。
//
// 版面以「設計單位」描述，實際像素等於設計單位乘上 Surface.Scale；本套件不查詢
// DPI，縮放倍率由前端算好傳入。
package ui
