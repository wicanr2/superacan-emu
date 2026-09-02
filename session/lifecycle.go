package session

import "github.com/wicanr2/superacan-emu/ui"

// 生命週期由入口送進來，處置寫在這裡而不是 ui：要落地的東西（卡帶電池記憶體、
// 設定）屬於主機端，ui 不碰檔案。
//
// 行動平台沒有「正常結束」這回事——使用者切走、系統回收，程式就沒了。所以離開
// 前景的那一刻就是最後一次能寫檔的機會，不能等到某個結束流程。

// Suspend 處理離開前景：先把該落地的寫出去，再叫出覆蓋選單。
//
// 叫出選單而不是只設一個暫停旗標，是因為回到前景時使用者要看得出來為什麼畫面不動，
// 而且要有一個明顯的「繼續遊戲」。凍住的畫面配上沒有任何說明的介面，看起來像當掉。
func (s *Session) Suspend() {
	s.flush()
	s.UI.Open()
}

// Resume 處理回到前景。這裡不自動恢復執行：由使用者按「繼續遊戲」。
// 自動恢復會讓人在還沒看清畫面時就被丟回操作中的遊戲。
func (s *Session) Resume() {}

// flush 把必須落地的狀態寫出去。設定與金手指在每次變更時就已經寫過，
// 這裡處理的是隨時在變、只在離開時寫的卡帶電池記憶體。
func (s *Session) flush() {
	if s.Flush == nil {
		return
	}
	if err := s.Flush(); err != nil {
		s.UI.Fail(err.Error())
	}
}

// handleLifecycle 回報這個事件有沒有在這一層被處理掉。LifeBack 不在這裡：
// 返回鍵是介面導覽，處置在 ui.handleBack。
func (s *Session) handleLifecycle(life ui.Life) bool {
	switch life.Kind {
	case ui.LifeSuspend, ui.LifeFocusLost:
		s.Suspend()
		return true
	case ui.LifeResume, ui.LifeFocusGained:
		s.Resume()
		return true
	}
	return false
}
