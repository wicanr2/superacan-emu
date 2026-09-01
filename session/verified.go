package session

// 已驗證清單。這些雜湊出自 docs/verify-rom-matrix.md 與 docs/verify-ui.md 記錄的
// 固定輸入，清單本身不含任何 ROM 或 BIOS 內容，只有雜湊。
//
// 「已知」只表示雜湊與本專案實際跑過的那一份相同。不在清單裡不代表不合法——
// 本專案沒有立場宣稱其他版本的韌體或卡帶不正確，畫面上因此顯示「未列於已驗證
// 清單」而不是錯誤。
var verifiedFirmware = map[string]string{
	"2e4d88bec69b5e7e4803368c233ce0d20f6dd107c5af0cfcc0089d310c695d7c": "internal_68k.bin",
	"f158d83be6e73389967c6dadfd5160bb742e09212a1b218fb829bae3b4961b28": "umc6650.bin",
	"219f51bcb8544fe733bf784e087544f97aa5457945260c7fa07a8639f30f3a68": "internal_6502_1.bin",
	"9889590805a97b7bb439d853d9ae4d6b31067bacb8225ab0538f3491dedab4b8": "internal_6502_2.bin",
}

// verifiedCartridges 的鍵是「解碼後的卡帶影像」雜湊，不是檔案雜湊：雙部分 ZIP
// 接合之後才是機器看到的內容。
var verifiedCartridges = map[string]string{
	"090827d00ef8047d2c78cc173d258565b1c3ab01f0d97dc3ed8e08833d370077": "Boom Zoo",
	"d6697e349613f70812cb7815de04bd89027d7e5b72471a981d16f6c667099b99": "Formosa Duel",
	"a4964b702214f70e199bb07bbd2777eb08875206fa27396989f2a79cb48c5087": "Journey to the Laugh",
	"b90b8dbfd15f1bcdd3e8f70910fa2f69effe07f2c781d04b123b738813fecb2f": "Monopoly - Adventure in Africa",
	"bb4f38089f8350a9f4005956b223300f8763f2ff9ca04d471329704d8e78e9f3": "Sango Fighter",
	"dfba00a46e7d71b9d78688bd902ec05e2c353f2ff119273d47b0a02602f3c9a2": "Speedy Dragon",
	"e0c17fcd21341c2416b19a830117db5898e6fd6995f41559bf7dd5ace745bd4e": "Super Taiwanese Baseball League",
	"791ab9d5ca182830fcf8ded488e71f1b61398da84967543396d0496e11bf5deb": "The Son of Evil",
	"1d79c19675b4577ee6208bae276d588577a894cd0311f7908fc151b2f6e6b0e3": "Super Dragon Force",
}
