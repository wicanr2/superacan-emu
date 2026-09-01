// Package session 把模擬核心與介面接在一起，並且是唯一執行 Intent 的地方。
//
// 分工：machine 只管硬體，ui 只管畫面與事件，session 負責在 frame 邊界之間
// 搬運。三個前端（headless、X11、Ebitengine，未來 macOS 與 Android）共用這一份，
// 所以「叫出選單、存檔、讀檔」這條流程可以在 headless 以畫面雜湊驗證，
// 不必靠人在視窗前面按一次。
//
// session 相依 machine 與 ui，但不相依任何前端套件。
package session
