//go:build cgo

package session

// cgoEnabled 回報這個 binary 有沒有連進 cgo。發行政策是 Linux 與 macOS 的
// binary 必須是 false，關於畫面因此要能誠實顯示。
const cgoEnabled = true
