package ui

import "reflect"

// Language 是 BCP 47 語言標籤。五種語言與 Bcan 相同。
type Language string

const (
	LanguageEnglish            Language = "en"
	LanguageFrench             Language = "fr"
	LanguageSpanish            Language = "es"
	LanguageTraditionalChinese Language = "zh-Hant"
	LanguageSimplifiedChinese  Language = "zh-Hans"
)

// Languages 是可選語言，順序即設定畫面上的順序。
var Languages = []Language{
	LanguageEnglish, LanguageFrench, LanguageSpanish,
	LanguageTraditionalChinese, LanguageSimplifiedChinese,
}

// LanguageNames 是各語言的自稱。語言選單一律用該語言自己的寫法，
// 使用者才找得到自己的語言。
var LanguageNames = map[Language]string{
	LanguageEnglish:            "English",
	LanguageFrench:             "Français",
	LanguageSpanish:            "Español",
	LanguageTraditionalChinese: "繁體中文",
	LanguageSimplifiedChinese:  "简体中文",
}

// translation 的欄位順序與 Languages 相同。技術標識（IPL、SHA-256、tilemap0、
// CHEAT、48,000 Hz 之類）五種語言相同，那不是沒翻譯，是不該翻。
type translation struct {
	key   string
	texts [5]string
}

var translations = []translation{
	{"Resume", [5]string{"Resume", "Continuer", "Continuar", "繼續遊戲", "继续游戏"}},
	{"SaveState", [5]string{"Save state", "Sauvegarder", "Guardar", "存檔", "存档"}},
	{"LoadState", [5]string{"Load state", "Charger", "Cargar", "讀檔", "读档"}},
	{"ResetMachine", [5]string{"Reset console", "Réinitialiser", "Reiniciar consola", "重設主機", "重设主机"}},
	{"Cheats", [5]string{"Cheats", "Codes", "Trucos", "金手指", "金手指"}},
	{"Settings", [5]string{"Settings", "Réglages", "Ajustes", "設定", "设定"}},
	{"Diagnostics", [5]string{"Diagnostics", "Diagnostic", "Diagnóstico", "診斷", "诊断"}},
	{"Screenshot", [5]string{"Screenshot", "Capture d'écran", "Captura", "截圖", "截图"}},
	{"EjectCart", [5]string{"Eject cartridge", "Éjecter la cartouche", "Expulsar cartucho", "退出卡帶", "退出卡带"}},
	{"Quit", [5]string{"Quit", "Quitter", "Salir", "離開模擬器", "离开模拟器"}},
	{"Paused", [5]string{"Paused", "En pause", "En pausa", "已暫停", "已暂停"}},
	{"NoCartridge", [5]string{"No cartridge", "Aucune cartouche", "Sin cartucho", "未載入卡帶", "未载入卡带"}},
	{"SlotPrefix", [5]string{"Slot ", "Emplacement ", "Ranura ", "槽 ", "槽 "}},
	{"Back", [5]string{"‹ Back", "‹ Retour", "‹ Atrás", "‹ 返回", "‹ 返回"}},
	{"ModeSave", [5]string{"Save", "Sauvegarder", "Guardar", "存檔", "存档"}},
	{"ModeLoad", [5]string{"Load", "Charger", "Cargar", "讀檔", "读档"}},
	{"SlotsTitle", [5]string{"Save slots", "Emplacements", "Ranuras", "存檔槽", "存档槽"}},
	{"EmptySlot", [5]string{"(empty)", "(vide)", "(vacío)", "（空）", "（空）"}},
	{"SlotLegend", [5]string{"✔ readable   ✖ rejected (validated before loading; current state is untouched)", "✔ lisible   ✖ refusé (vérifié avant chargement ; l'état actuel reste intact)", "✔ legible   ✖ rechazado (verificado antes de cargar; el estado actual no cambia)", "✔ 可讀取   ✖ 已拒絕（載入前驗證失敗，現行狀態不會被更動）", "✔ 可读取   ✖ 已拒绝（载入前验证失败，现行状态不会被更动）"}},
	{"SlotKeys", [5]string{"Enter run · Del delete · ←→↑↓ select", "Entrée exécuter · Suppr supprimer · ←→↑↓ choisir", "Intro ejecutar · Supr borrar · ←→↑↓ elegir", "Enter 執行 · Del 刪除 · ←→↑↓ 選擇", "Enter 执行 · Del 删除 · ←→↑↓ 选择"}},
	{"Cancel", [5]string{"Cancel", "Annuler", "Cancelar", "取消", "取消"}},
	{"Overwrite", [5]string{"Overwrite", "Écraser", "Sobrescribir", "覆寫", "覆写"}},
	{"Delete", [5]string{"Delete", "Supprimer", "Borrar", "刪除", "删除"}},
	{"DismissError", [5]string{"Enter to dismiss", "Entrée pour fermer", "Intro para cerrar", "Enter 關閉", "Enter 关闭"}},
	{"SoftReset", [5]string{"Soft reset", "Réinitialisation logicielle", "Reinicio suave", "軟重設", "软重设"}},
	{"ColdReset", [5]string{"Cold boot", "Démarrage à froid", "Arranque en frío", "冷開機", "冷开机"}},
	{"NotYet", [5]string{"Not implemented yet: %s", "Pas encore implémenté : %s", "Aún no implementado: %s", "這一項還沒做：%s", "这一项还没做：%s"}},
	{"StageCheats", [5]string{"cheats land in P6", "les codes arrivent en P6", "los trucos llegan en P6", "金手指在 P6", "金手指在 P6"}},
	{"StageSettings", [5]string{"settings land in P3 and P5", "les réglages arrivent en P3 et P5", "los ajustes llegan en P3 y P5", "設定在 P3 與 P5", "设定在 P3 与 P5"}},
	{"StageDiag", [5]string{"diagnostics land in P5", "le diagnostic arrive en P5", "el diagnóstico llega en P5", "診斷在 P5", "诊断在 P5"}},
	{"Saved", [5]string{"Written to slot %d.", "Écrit dans l'emplacement %d.", "Guardado en la ranura %d.", "已寫入存檔槽 %d。", "已写入存档槽 %d。"}},
	{"Loaded", [5]string{"Loaded slot %d.", "Emplacement %d chargé.", "Ranura %d cargada.", "已讀取存檔槽 %d。", "已读取存档槽 %d。"}},
	{"Deleted", [5]string{"Deleted slot %d.", "Emplacement %d supprimé.", "Ranura %d borrada.", "已刪除存檔槽 %d。", "已删除存档槽 %d。"}},
	{"SlotEmpty", [5]string{"Slot %d is empty.", "L'emplacement %d est vide.", "La ranura %d está vacía.", "存檔槽 %d 是空的。", "存档槽 %d 是空的。"}},
	{"SlotRejected", [5]string{"Slot %d cannot be read: %s", "L'emplacement %d est illisible : %s", "No se puede leer la ranura %d: %s", "存檔槽 %d 無法讀取：%s", "存档槽 %d 无法读取：%s"}},
	{"OverwriteAsk", [5]string{"Overwrite slot %d?", "Écraser l'emplacement %d ?", "¿Sobrescribir la ranura %d?", "覆寫存檔槽 %d？", "覆写存档槽 %d？"}},
	{"OverwriteWhy", [5]string{"Slot %d holds a save from %s at frame %d. Overwriting cannot be undone.", "L'emplacement %d contient une sauvegarde de %s à l'image %d. L'écrasement est définitif.", "La ranura %d tiene una partida de %s en el fotograma %d. Sobrescribir es irreversible.", "槽 %d 目前是 %s 的存檔，frame %d。覆寫後無法復原。", "槽 %d 目前是 %s 的存档，frame %d。覆写后无法复原。"}},
	{"DeleteAsk", [5]string{"Delete slot %d?", "Supprimer l'emplacement %d ?", "¿Borrar la ranura %d?", "刪除存檔槽 %d？", "删除存档槽 %d？"}},
	{"DeleteWhy", [5]string{"The %s save in slot %d will be deleted. This cannot be undone.", "La sauvegarde %s de l'emplacement %d sera supprimée. C'est définitif.", "Se borrará la partida %s de la ranura %d. Es irreversible.", "槽 %d 的 %s 存檔會被刪除，無法復原。", "槽 %d 的 %s 存档会被删除，无法复原。"}},
	{"QuitAsk", [5]string{"Quit the emulator?", "Quitter l'émulateur ?", "¿Salir del emulador?", "離開模擬器？", "离开模拟器？"}},
	{"QuitWhy", [5]string{"Cartridge battery memory is written back before exiting.", "La mémoire de sauvegarde de la cartouche est écrite avant de quitter.", "La memoria de batería del cartucho se guarda antes de salir.", "卡帶電池記憶體會先寫回再結束。", "卡带电池记忆体会先写回再结束。"}},
	{"ScreenshotHK", [5]string{"F8", "F8", "F8", "F8", "F8"}},
	{"AppTitle", [5]string{"SUPER A'CAN", "SUPER A'CAN", "SUPER A'CAN", "SUPER A'CAN", "SUPER A'CAN"}},
	{"SectionFirmware", [5]string{"Console firmware", "Micrologiciel", "Firmware de la consola", "主機韌體", "主机韧体"}},
	{"SectionRecent", [5]string{"Recent cartridges", "Cartouches récentes", "Cartuchos recientes", "最近的卡帶", "最近的卡带"}},
	{"FirmwareIPL", [5]string{"IPL", "IPL", "IPL", "IPL", "IPL"}},
	{"FirmwareKey", [5]string{"UMC6650 key", "UMC6650 key", "UMC6650 key", "UMC6650 key", "UMC6650 key"}},
	{"FirmwareSoundA", [5]string{"Sound BIOS 1", "BIOS audio 1", "BIOS de sonido 1", "音效 BIOS 1", "音效 BIOS 1"}},
	{"FirmwareSoundB", [5]string{"Sound BIOS 2", "BIOS audio 2", "BIOS de sonido 2", "音效 BIOS 2", "音效 BIOS 2"}},
	{"FirmwareLoaded", [5]string{"loaded", "chargé", "cargado", "已載入", "已载入"}},
	{"FirmwareMissing", [5]string{"missing", "manquant", "falta", "缺少", "缺少"}},
	{"FirmwareUnset", [5]string{"not set", "non défini", "sin definir", "未設定", "未设定"}},
	{"FirmwareKnown", [5]string{"✔ known", "✔ connu", "✔ conocido", "✔ 已知", "✔ 已知"}},
	{"FirmwareUnlisted", [5]string{"not on the verified list", "absent de la liste vérifiée", "no está en la lista verificada", "未列於已驗證清單", "未列于已验证清单"}},
	{"FirmwareSetUp", [5]string{"Set up console firmware…", "Configurer le micrologiciel…", "Configurar firmware…", "設定主機韌體…", "设定主机韧体…"}},
	{"FirmwareTitle", [5]string{"Console firmware", "Micrologiciel", "Firmware de la consola", "主機韌體", "主机韧体"}},
	{"FirmwareNotice", [5]string{"You supply these four files yourself; they are not distributed with this program.", "Ces quatre fichiers vous appartiennent ; ils ne sont pas distribués avec ce programme.", "Usted aporta estos cuatro archivos; no se distribuyen con este programa.", "這四個檔案由使用者自行提供，不隨本程式散布。", "这四个档案由使用者自行提供，不随本程式散布。"}},
	{"FirmwareIncompl", [5]string{"Firmware incomplete: no cartridge starts while any file is missing.", "Micrologiciel incomplet : aucune cartouche ne démarre s'il manque un fichier.", "Firmware incompleto: ningún cartucho arranca si falta algún archivo.", "韌體不齊：缺少任一檔案時不會啟動任何卡帶。", "韧体不齐：缺少任一档案时不会启动任何卡带。"}},
	{"ChooseCartridge", [5]string{"Choose cartridge…", "Choisir une cartouche…", "Elegir cartucho…", "選擇卡帶…", "选择卡带…"}},
	{"About", [5]string{"About", "À propos", "Acerca de", "關於", "关于"}},
	{"AboutTitle", [5]string{"About", "À propos", "Acerca de", "關於", "关于"}},
	{"AboutName", [5]string{"Super A'Can emulator", "Émulateur Super A'Can", "Emulador Super A'Can", "Super A'Can 模擬器", "Super A'Can 模拟器"}},
	{"AboutDisclaimer", [5]string{"This program contains and distributes no ROM, BIOS or game imagery. You supply every cartridge and firmware file yourself.", "Ce programme ne contient ni ne distribue aucune ROM, BIOS ou image de jeu. Vous fournissez vous-même chaque cartouche et micrologiciel.", "Este programa no contiene ni distribuye ninguna ROM, BIOS o imagen de juego. Usted aporta cada cartucho y firmware.", "本程式不含、也不散布任何 ROM、BIOS 或遊戲畫面。所有卡帶與韌體檔案由使用者自行提供。", "本程式不含、也不散布任何 ROM、BIOS 或游戏画面。所有卡带与韧体档案由使用者自行提供。"}},
	{"AboutThirdParty", [5]string{"Third-party components", "Composants tiers", "Componentes de terceros", "第三方元件", "第三方元件"}},
	{"AboutMAME", [5]string{"Some device behaviour references MAME (BSD-3-Clause); attributions are in the source comments.", "Certains comportements matériels s'appuient sur MAME (BSD-3-Clause) ; les mentions sont dans les commentaires du code.", "Parte del comportamiento del hardware se basa en MAME (BSD-3-Clause); las atribuciones están en los comentarios del código.", "部分裝置行為參考 MAME（BSD-3-Clause），標示見原始碼註解。", "部分装置行为参考 MAME（BSD-3-Clause），标示见原始码注解。"}},
	{"CGOEnabled", [5]string{"cgo enabled", "cgo activé", "cgo activado", "cgo 啟用", "cgo 启用"}},
	{"CGODisabled", [5]string{"cgo disabled", "cgo désactivé", "cgo desactivado", "cgo 停用", "cgo 停用"}},
	{"BrowserTitle", [5]string{"Cartridges", "Cartouches", "Cartuchos", "卡帶", "卡带"}},
	{"BrowserNoPreview", [5]string{"(no preview: this program does not distribute game imagery)", "(pas d'aperçu : ce programme ne distribue pas d'images de jeu)", "(sin vista previa: este programa no distribuye imágenes de juego)", "（無預覽：本程式不散布遊戲畫面）", "（无预览：本程式不散布游戏画面）"}},
	{"BrowserEmpty", [5]string{"No cartridge files in this directory.", "Aucun fichier de cartouche dans ce dossier.", "No hay archivos de cartucho en esta carpeta.", "這個目錄沒有卡帶檔案。", "这个目录没有卡带档案。"}},
	{"FieldSize", [5]string{"Size", "Taille", "Tamaño", "大小", "大小"}},
	{"FieldKind", [5]string{"Type", "Type", "Tipo", "類型", "类型"}},
	{"FieldSHA", [5]string{"SHA-256", "SHA-256", "SHA-256", "SHA-256", "SHA-256"}},
	{"FieldSaves", [5]string{"Saves", "Sauvegardes", "Partidas", "存檔", "存档"}},
	{"FieldBattery", [5]string{"Battery", "Pile", "Batería", "電池", "电池"}},
	{"FieldCompat", [5]string{"Compatibility", "Compatibilité", "Compatibilidad", "相容性", "兼容性"}},
	{"CompatVerified", [5]string{"verified to 1200 frames", "vérifié sur 1200 images", "verificado hasta 1200 fotogramas", "1200 幀已驗證", "1200 帧已验证"}},
	{"CompatUnverified", [5]string{"unverified", "non vérifié", "sin verificar", "未驗證", "未验证"}},
	{"LoadCartridge", [5]string{"Load", "Charger", "Cargar", "載入", "载入"}},
	{"BrowserKeys", [5]string{"Enter load · ↑↓ select · Esc back", "Entrée charger · ↑↓ choisir · Échap retour", "Intro cargar · ↑↓ elegir · Esc atrás", "Enter 載入 · ↑↓ 選擇 · Esc 返回", "Enter 载入 · ↑↓ 选择 · Esc 返回"}},
	{"MissingFile", [5]string{"(file moved or deleted)", "(fichier déplacé ou supprimé)", "(archivo movido o borrado)", "(檔案已移動或刪除)", "(档案已移动或删除)"}},
	{"StatusNoCartridge", [5]string{"No cartridge loaded. Set up the console firmware first, then choose a cartridge.", "Aucune cartouche chargée. Configurez d'abord le micrologiciel, puis choisissez une cartouche.", "Sin cartucho. Configure primero el firmware y luego elija un cartucho.", "尚未載入卡帶。先完成主機韌體設定，再選擇卡帶。", "尚未载入卡带。先完成主机韧体设定，再选择卡带。"}},
	{"StatusReady", [5]string{"Firmware complete. Choose a cartridge to begin.", "Micrologiciel complet. Choisissez une cartouche.", "Firmware completo. Elija un cartucho.", "韌體齊備。選擇一個卡帶開始。", "韧体齐备。选择一个卡带开始。"}},
	{"HaltTitle", [5]string{"✖ Stopped", "✖ Arrêté", "✖ Detenido", "✖ 已停止", "✖ 已停止"}},
	{"HaltBody", [5]string{"Execution did not continue; the machine state is kept for inspection.", "L'exécution ne continue pas ; l'état de la machine est conservé pour inspection.", "La ejecución no continuó; se conserva el estado de la máquina para inspección.", "執行沒有繼續，機器狀態保留供檢視。", "执行没有继续，机器状态保留供检视。"}},
	{"HaltFrame", [5]string{"frame", "frame", "frame", "frame", "frame"}},
	{"HaltInstructions", [5]string{"68000 instructions", "instructions 68000", "instrucciones 68000", "68000 指令", "68000 指令"}},
	{"HaltCartridge", [5]string{"Cartridge", "Cartouche", "Cartucho", "卡帶", "卡带"}},
	{"HaltIPL", [5]string{"IPL", "IPL", "IPL", "IPL", "IPL"}},
	{"None", [5]string{"—", "—", "—", "—", "—"}},
	{"EjectToShell", [5]string{"Eject cartridge", "Éjecter la cartouche", "Expulsar cartucho", "退出卡帶", "退出卡带"}},
	{"SettingsTitle", [5]string{"Settings", "Réglages", "Ajustes", "設定", "设定"}},
	{"SettingsInput", [5]string{"Input", "Commandes", "Controles", "輸入", "输入"}},
	{"SettingsHotkeys", [5]string{"Hotkeys", "Raccourcis", "Atajos", "熱鍵", "热键"}},
	{"SettingsVideo", [5]string{"Video", "Vidéo", "Vídeo", "影像", "影像"}},
	{"SettingsAudio", [5]string{"Audio", "Audio", "Audio", "音訊", "音讯"}},
	{"SettingsLanguage", [5]string{"Language", "Langue", "Idioma", "語言", "语言"}},
	{"StageVideo", [5]string{"video settings land in P5", "les réglages vidéo arrivent en P5", "los ajustes de vídeo llegan en P5", "影像設定在 P5", "影像设定在 P5"}},
	{"StageAudio", [5]string{"audio settings land in P5", "les réglages audio arrivent en P5", "los ajustes de audio llegan en P5", "音訊設定在 P5", "音讯设定在 P5"}},
	{"StageLanguage", [5]string{"language lands in P8", "la langue arrive en P8", "el idioma llega en P8", "語言在 P8", "语言在 P8"}},
	{"InputTitle", [5]string{"Input", "Commandes", "Controles", "輸入", "输入"}},
	{"HotkeyTitle", [5]string{"Hotkeys", "Raccourcis", "Atajos", "熱鍵", "热键"}},
	{"ColumnButton", [5]string{"Button", "Bouton", "Botón", "按鈕", "按钮"}},
	{"ColumnAction", [5]string{"Action", "Action", "Acción", "動作", "动作"}},
	{"ColumnKeyboard", [5]string{"Keyboard", "Clavier", "Teclado", "鍵盤", "键盘"}},
	{"ColumnGamepad", [5]string{"Gamepad", "Manette", "Mando", "手把", "手把"}},
	{"PressInput", [5]string{"Press a key… (Esc cancels)", "Appuyez sur une touche… (Échap annule)", "Pulse una tecla… (Esc cancela)", "按下按鍵…（Esc 取消）", "按下按键…（Esc 取消）"}},
	{"ConflictWith", [5]string{"shared with “%s”", "partagé avec « %s »", "compartido con «%s»", "與「%s」共用", "与「%s」共用"}},
	{"InputHelp", [5]string{"Enter assign · Del clear · ←→ keyboard/gamepad · Tab P1/P2", "Entrée assigner · Suppr effacer · ←→ clavier/manette · Tab J1/J2", "Intro asignar · Supr borrar · ←→ teclado/mando · Tab J1/J2", "Enter 指定 · Del 清除 · ←→ 切換鍵盤／手把 · Tab 切換 P1／P2", "Enter 指定 · Del 清除 · ←→ 切换键盘／手把 · Tab 切换 P1／P2"}},
	{"HotkeyHelp", [5]string{"Enter assign · Del clear · Esc back", "Entrée assigner · Suppr effacer · Échap retour", "Intro asignar · Supr borrar · Esc atrás", "Enter 指定 · Del 清除 · Esc 返回", "Enter 指定 · Del 清除 · Esc 返回"}},
	{"HotkeyConflict", [5]string{"※ Two hotkeys point at the same key; both actions will fire.", "※ Deux raccourcis pointent la même touche ; les deux actions se déclencheront.", "※ Dos atajos apuntan a la misma tecla; se dispararán ambas acciones.", "※ 有熱鍵指到同一個按鍵，兩個動作都會觸發。", "※ 有热键指到同一个按键，两个动作都会触发。"}},
	{"On", [5]string{"on", "activé", "activado", "開啟", "开启"}},
	{"Off", [5]string{"off", "désactivé", "desactivado", "關閉", "关闭"}},
	{"VideoTitle", [5]string{"Video", "Vidéo", "Vídeo", "影像", "影像"}},
	{"AudioTitle", [5]string{"Audio", "Audio", "Audio", "音訊", "音讯"}},
	{"DiagTitle", [5]string{"Diagnostics", "Diagnostic", "Diagnóstico", "診斷", "诊断"}},
	{"VideoScale", [5]string{"Scale", "Échelle", "Escala", "縮放", "缩放"}},
	{"VideoInteger", [5]string{"Integer scaling", "Échelle entière", "Escala entera", "整數縮放", "整数缩放"}},
	{"VideoAspect", [5]string{"Aspect ratio", "Format d'image", "Relación de aspecto", "長寬比", "长宽比"}},
	{"VideoFilter", [5]string{"Filter", "Filtre", "Filtro", "濾鏡", "滤镜"}},
	{"VideoFullscreen", [5]string{"Fullscreen", "Plein écran", "Pantalla completa", "全螢幕", "全萤幕"}},
	{"VideoFrameBlend", [5]string{"Motion blending", "Fondu d'images", "Mezcla de fotogramas", "動態平滑", "动态平滑"}},
	{"VideoShowFPS", [5]string{"Show FPS", "Afficher les FPS", "Mostrar FPS", "顯示 FPS", "显示 FPS"}},
	{"VideoSuppress", [5]string{"Suppress action messages", "Masquer les messages d'action", "Ocultar mensajes de acción", "抑制操作訊息", "抑制操作讯息"}},
	{"VideoSuppressNote", [5]string{"errors are shown regardless", "les erreurs restent affichées", "los errores se muestran igualmente", "錯誤訊息不受此設定影響", "错误讯息不受此设定影响"}},
	{"VideoApertureNote", [5]string{"Cartridges in 256-column mode look narrower under “1:1 pixels”; that is the display aperture, not a fault.", "Les cartouches en mode 256 colonnes paraissent plus étroites en « pixels 1:1 » ; c'est l'ouverture d'affichage, pas un défaut.", "Los cartuchos en modo de 256 columnas se ven más estrechos con «píxeles 1:1»; es la apertura de pantalla, no un fallo.", "256 欄模式的卡帶在「1:1 像素」下畫面較窄，這是顯示孔徑造成的。", "256 栏模式的卡带在「1:1 像素」下画面较窄，这是显示孔径造成的。"}},
	{"StageFrameBlend", [5]string{"motion blending is not implemented", "le fondu d'images n'est pas implémenté", "la mezcla de fotogramas no está implementada", "動態平滑尚未實作", "动态平滑尚未实作"}},
	{"AudioVolume", [5]string{"Master volume", "Volume principal", "Volumen principal", "主音量", "主音量"}},
	{"AudioMuteFast", [5]string{"Mute on fast forward", "Muet en avance rapide", "Silencio en avance rápido", "全速時靜音", "全速时静音"}},
	{"AudioBuffer", [5]string{"Output buffer", "Tampon de sortie", "Búfer de salida", "輸出緩衝", "输出缓冲"}},
	{"AudioBufferNote", [5]string{"oldest samples are dropped on overflow", "les plus anciens échantillons sont abandonnés en cas de dépassement", "al desbordarse se descartan las muestras más antiguas", "溢位時丟棄最舊樣本", "溢位时丢弃最旧样本"}},
	{"AudioSink", [5]string{"Output", "Sortie", "Salida", "輸出方式", "输出方式"}},
	{"AudioFormat", [5]string{"Sample format", "Format d'échantillon", "Formato de muestra", "取樣率", "取样率"}},
	{"AudioFormatValue", [5]string{"48,000 Hz · 16-bit · stereo", "48,000 Hz · 16-bit · stereo", "48,000 Hz · 16-bit · stereo", "48,000 Hz · 16-bit · stereo", "48,000 Hz · 16-bit · stereo"}},
	{"AudioBufferState", [5]string{"Buffer state", "État du tampon", "Estado del búfer", "緩衝狀態", "缓冲状态"}},
	{"AudioUnderrun", [5]string{"underrun", "underrun", "underrun", "underrun", "underrun"}},
	{"AudioNote", [5]string{"Volume and buffering affect host playback only; they do not change UM6619 sampling or machine timing.", "Le volume et le tampon n'affectent que la lecture hôte ; ils ne changent ni l'échantillonnage UM6619 ni la temporisation.", "El volumen y el búfer solo afectan a la reproducción del anfitrión; no cambian el muestreo del UM6619 ni la temporización.", "音量與緩衝只影響主機播放，不改變 UM6619 的取樣或機器時序。", "音量与缓冲只影响主机播放，不改变 UM6619 的取样或机器时序。"}},
	{"AudioNoSink", [5]string{"not set (this frontend has no built-in player)", "non défini (ce frontal n'a pas de lecteur intégré)", "sin definir (este frontal no tiene reproductor integrado)", "未設定（本前端沒有內建播放器）", "未设定（本前端没有内建播放器）"}},
	{"DiagFrame", [5]string{"frame", "frame", "frame", "frame", "frame"}},
	{"DiagM68K", [5]string{"68000 instructions", "instructions 68000", "instrucciones 68000", "68000 指令", "68000 指令"}},
	{"DiagM65C02", [5]string{"65C02 instructions", "instructions 65C02", "instrucciones 65C02", "65C02 指令", "65C02 指令"}},
	{"DiagIRQ7", [5]string{"vblank IRQ7 taken", "IRQ7 vblank acceptées", "IRQ7 de vblank aceptadas", "vblank IRQ7 受理", "vblank IRQ7 受理"}},
	{"DiagIRQ45", [5]string{"IRQ4 / IRQ5", "IRQ4 / IRQ5", "IRQ4 / IRQ5", "IRQ4 / IRQ5", "IRQ4 / IRQ5"}},
	{"DiagClash", [5]string{"sound clash", "sound clash", "sound clash", "sound clash", "sound clash"}},
	{"DiagFrontend", [5]string{"Frontend", "Frontal", "Frontal", "前端", "前端"}},
	{"DiagCGO", [5]string{"cgo", "cgo", "cgo", "cgo", "cgo"}},
	{"DiagLayerMask", [5]string{"Layer mask", "Masque de couches", "Máscara de capas", "圖層遮罩", "图层遮罩"}},
	{"DiagLayerNote", [5]string{"The layer mask only affects framebuffer composition; it does not change instruction counts or hardware timing.", "Le masque de couches n'affecte que la composition du framebuffer ; il ne change ni le nombre d'instructions ni la temporisation.", "La máscara de capas solo afecta a la composición del framebuffer; no cambia el conteo de instrucciones ni la temporización.", "圖層遮罩只影響 framebuffer 合成，不影響指令數與硬體時序。", "图层遮罩只影响 framebuffer 合成，不影响指令数与硬体时序。"}},
	{"DiagMaskWarning", [5]string{"A masked frame hash must not be used for comparison.", "Un hachage d'image masquée ne doit pas servir de référence.", "Un hash de imagen enmascarada no debe usarse para comparar.", "套了遮罩的畫面雜湊不可拿來對帳。", "套了遮罩的画面杂凑不可拿来对帐。"}},
	{"LayerTilemap0", [5]string{"tilemap0", "tilemap0", "tilemap0", "tilemap0", "tilemap0"}},
	{"LayerTilemap1", [5]string{"tilemap1", "tilemap1", "tilemap1", "tilemap1", "tilemap1"}},
	{"LayerTilemap2", [5]string{"tilemap2", "tilemap2", "tilemap2", "tilemap2", "tilemap2"}},
	{"LayerSprite", [5]string{"sprite", "sprite", "sprite", "sprite", "sprite"}},
	{"LayerROZ", [5]string{"ROZ", "ROZ", "ROZ", "ROZ", "ROZ"}},
	{"LayerWindow", [5]string{"window", "window", "window", "window", "window"}},
	{"CheatTitle", [5]string{"Cheats", "Codes", "Trucos", "金手指", "金手指"}},
	{"CheatRange", [5]string{"Range", "Plage", "Rango", "範圍", "范围"}},
	{"CheatRangeValue", [5]string{"Work RAM $FC0000–$FCFFFF (64 KiB, fixed)", "RAM de travail $FC0000–$FCFFFF (64 Kio, fixe)", "RAM de trabajo $FC0000–$FCFFFF (64 KiB, fija)", "Work RAM $FC0000–$FCFFFF（64 KiB，固定）", "Work RAM $FC0000–$FCFFFF（64 KiB，固定）"}},
	{"CheatWidth", [5]string{"Width", "Largeur", "Ancho", "寬度", "宽度"}},
	{"CheatCompare", [5]string{"Compare", "Comparaison", "Comparación", "比較", "比较"}},
	{"CheatValue", [5]string{"Value", "Valeur", "Valor", "數值", "数值"}},
	{"CheatNewSearch", [5]string{"New search", "Nouvelle recherche", "Nueva búsqueda", "開始新搜尋", "开始新搜寻"}},
	{"CheatRefine", [5]string{"Refine", "Affiner", "Refinar", "縮小範圍", "缩小范围"}},
	{"CheatClear", [5]string{"Clear", "Effacer", "Limpiar", "清除", "清除"}},
	{"CheatCandidates", [5]string{"Candidate addresses: %d (showing %d)", "Adresses candidates : %d (affichées %d)", "Direcciones candidatas: %d (mostrando %d)", "候選位址：%d（顯示 %d）", "候选位址：%d（显示 %d）"}},
	{"CheatRefines", [5]string{" · refined %d times · snapshot taken at a frame boundary", " · affiné %d fois · instantané pris à une frontière d'image", " · refinado %d veces · instantánea tomada en un límite de fotograma", " · 第 %d 次縮小 · 快照取自 frame 邊界", " · 第 %d 次缩小 · 快照取自 frame 边界"}},
	{"CheatPrevious", [5]string{"was %d", "avant %d", "antes %d", "前次 %d", "前次 %d"}},
	{"CheatTruncNote", [5]string{"Above 4096 candidates only the first 4096 are listed; refine once more.", "Au-delà de 4096 candidats, seuls les 4096 premiers sont listés ; affinez encore.", "Por encima de 4096 candidatos solo se listan los primeros 4096; refine otra vez.", "候選超過 4096 時只顯示前 4096 筆；請再縮小一次範圍。", "候选超过 4096 时只显示前 4096 笔；请再缩小一次范围。"}},
	{"CheatEnable", [5]string{"Enable cheats", "Activer les codes", "Activar trucos", "啟用金手指", "启用金手指"}},
	{"CheatEnableHint", [5]string{"X toggles · Tab switches to search", "X bascule · Tab passe à la recherche", "X alterna · Tab cambia a búsqueda", "X 切換 · Tab 切到搜尋", "X 切换 · Tab 切到搜寻"}},
	{"CheatColumnLock", [5]string{"Lock", "Verrou", "Bloqueo", "鎖", "锁"}},
	{"CheatColumnName", [5]string{"Name", "Nom", "Nombre", "名稱", "名称"}},
	{"CheatColumnAddress", [5]string{"Address", "Adresse", "Dirección", "位址", "位址"}},
	{"CheatColumnWidth", [5]string{"Width", "Largeur", "Ancho", "寬度", "宽度"}},
	{"CheatColumnValue", [5]string{"Value", "Valeur", "Valor", "值", "值"}},
	{"CheatColumnFormat", [5]string{"Format", "Format", "Formato", "格式", "格式"}},
	{"CheatEmpty", [5]string{"The list is empty. Find an address on the search page and add it.", "La liste est vide. Trouvez une adresse dans la recherche et ajoutez-la.", "La lista está vacía. Busque una dirección y añádala.", "清單是空的。到搜尋頁找出位址再加入。", "清单是空的。到搜寻页找出位址再加入。"}},
	{"CheatLockNote", [5]string{"Locked entries are written to Work RAM once per frame boundary.", "Les entrées verrouillées sont écrites en RAM de travail à chaque frontière d'image.", "Las entradas bloqueadas se escriben en la RAM de trabajo una vez por fotograma.", "鎖定項在每個 frame 邊界寫入一次 Work RAM。", "锁定项在每个 frame 边界写入一次 Work RAM。"}},
	{"CheatEvidenceWarning", [5]string{"Instruction counts and frame hashes from this session cannot be used as hardware evidence.", "Le nombre d'instructions et les hachages d'image de cette session ne valent pas comme preuve matérielle.", "El conteo de instrucciones y los hashes de esta sesión no sirven como evidencia de hardware.", "啟用期間的指令數與畫面雜湊不可作為硬體證據。", "启用期间的指令数与画面杂凑不可作为硬体证据。"}},
	{"CheatActive", [5]string{"※ memory writing enabled", "※ écriture mémoire activée", "※ escritura de memoria activada", "※ 已啟用記憶體寫入", "※ 已启用记忆体写入"}},
	{"CheatMarker", [5]string{"CHEAT", "CHEAT", "CHEAT", "CHEAT", "CHEAT"}},
	{"MenuHint", [5]string{"%s  Menu", "%s  Menu", "%s  Menú", "%s  選單", "%s  菜单"}},
	{"CaptureStart", [5]string{"Start recording", "Démarrer l'enregistrement", "Iniciar grabación", "開始錄影", "开始录影"}},
	{"CaptureStop", [5]string{"Stop recording", "Arrêter l'enregistrement", "Detener grabación", "停止錄影", "停止录影"}},
	{"CaptureHK", [5]string{"F9", "F9", "F9", "F9", "F9"}},
	{"CaptureFrames", [5]string{"%d frames", "%d images", "%d fotogramas", "%d 幀", "%d 帧"}},
	{"CaptureStarted", [5]string{"Recording started.", "Enregistrement démarré.", "Grabación iniciada.", "已開始錄影。", "已开始录影。"}},
	{"CaptureStopped", [5]string{"Recording stopped.", "Enregistrement arrêté.", "Grabación detenida.", "已停止錄影。", "已停止录影。"}},
	{"ScreenshotSaved", [5]string{"Screenshot written.", "Capture d'écran écrite.", "Captura guardada.", "已寫出截圖。", "已写出截图。"}},
	{"TouchTitle", [5]string{"Touch layout", "Disposition tactile", "Diseño táctil", "觸控版面", "触控版面"}},
	{"TouchOpacity", [5]string{"Opacity", "Opacité", "Opacidad", "不透明度", "不透明度"}},
	{"TouchScale", [5]string{"Button size", "Taille des boutons", "Tamaño de botones", "按鍵大小", "按键大小"}},
	{"TouchDeadzone", [5]string{"D-Pad dead zone", "Zone morte du D-Pad", "Zona muerta del D-Pad", "方向鍵死區", "方向键死区"}},
	{"TouchDeadzoneNote", [5]string{"too small turns “up” into “up+left”", "trop petite, « haut » devient « haut+gauche »", "demasiado pequeña convierte «arriba» en «arriba+izquierda»", "太小會讓「按上」變成「上＋左」", "太小会让「按上」变成「上＋左」"}},
	{"TouchSwapHands", [5]string{"Swap hands", "Inverser les mains", "Intercambiar manos", "左右手互換", "左右手互换"}},
	{"TouchStickMode", [5]string{"Stick mode", "Mode stick", "Modo stick", "搖桿模式", "摇杆模式"}},
	{"StageStickMode", [5]string{"stick mode is not implemented", "le mode stick n'est pas implémenté", "el modo stick no está implementado", "搖桿模式尚未實作", "摇杆模式尚未实作"}},
	{"TouchHaptics", [5]string{"Haptics", "Retour haptique", "Vibración", "觸覺回饋", "触觉回馈"}},
	{"HotkeyToast", [5]string{"%s: %s", "%s : %s", "%s: %s", "%s：%s", "%s：%s"}},
	{"HotkeyResumed", [5]string{"Resumed", "Reprise", "Reanudado", "已繼續", "已继续"}},
	{"HotkeyFastForward", [5]string{"Fast forward", "Avance rapide", "Avance rápido", "全速", "全速"}},
	{"HotkeyMute", [5]string{"Mute", "Silence", "Silencio", "靜音", "静音"}},
	{"HotkeySlot", [5]string{"Slot %d", "Emplacement %d", "Ranura %d", "存檔槽 %d", "存档槽 %d"}},
	{"TouchNote", [5]string{"The overlay menu hides the virtual pad: two control schemes at once fight over the same touches.", "Le menu masque la manette virtuelle : deux schémas de contrôle se disputeraient les mêmes contacts.", "El menú oculta el mando virtual: dos esquemas de control competirían por los mismos toques.", "覆蓋選單開著時虛擬手把隱藏：兩套控制同時存在會互相搶觸點。", "覆盖选单开着时虚拟手把隐藏：两套控制同时存在会互相抢触点。"}},
}

// stringsFor 以反射把語言表填進 Strings。表是逐列的，結構是逐欄的，
// 反射讓兩者只需要對得上欄位名稱，不必維護第二份順序。
func stringsFor(language Language) Strings {
	column := 0
	for index, candidate := range Languages {
		if candidate == language {
			column = index
		}
	}
	var out Strings
	value := reflect.ValueOf(&out).Elem()
	for _, row := range translations {
		field := value.FieldByName(row.key)
		if !field.IsValid() {
			continue
		}
		field.SetString(row.texts[column])
	}
	return out
}
