// Package x11 是純 Go 的 X11 呈現層，讓 Linux 桌面前端不需要 cgo。
//
// 它只做三件事：把 UM6618 的 ARGB framebuffer 以整數倍放大後送上視窗、
// 讀取鍵盤狀態、回報視窗關閉。所有硬體時序仍由 machine 決定，這一層不回饋
// 任何狀態給模擬核心。
package x11

import (
	"fmt"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/xproto"
)

// Window 是一個已映射的 X11 視窗與它的繪圖脈絡。
type Window struct {
	conn       *xgb.Conn
	window     xproto.Window
	gc         xproto.Gcontext
	depth      byte
	sourceW    int
	sourceH    int
	scale      int
	image      []byte
	pressed    map[xproto.Keycode]bool
	keysyms    map[xproto.Keycode]uint32
	closed     bool
	maxRequest int
}

// New 建立並映射一個 sourceW×sourceH 以 scale 倍放大的視窗。
func New(title string, sourceW, sourceH, scale int) (*Window, error) {
	if scale < 1 {
		return nil, fmt.Errorf("x11: scale must be at least 1")
	}
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("x11: connect: %w", err)
	}
	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)

	window, err := xproto.NewWindowId(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("x11: window id: %w", err)
	}
	width := uint16(sourceW * scale)
	height := uint16(sourceH * scale)
	err = xproto.CreateWindowChecked(conn, screen.RootDepth, window, screen.Root,
		0, 0, width, height, 0, xproto.WindowClassInputOutput, screen.RootVisual,
		xproto.CwBackPixel|xproto.CwEventMask,
		[]uint32{screen.BlackPixel, xproto.EventMaskKeyPress | xproto.EventMaskKeyRelease |
			xproto.EventMaskExposure | xproto.EventMaskStructureNotify}).Check()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("x11: create window: %w", err)
	}

	// WM_DELETE_WINDOW 讓關閉按鈕變成事件而不是直接斷線。
	if protocols, delete, ok := deleteAtoms(conn); ok {
		_ = xproto.ChangePropertyChecked(conn, xproto.PropModeReplace, window, protocols,
			xproto.AtomAtom, 32, 1, atomBytes(delete)).Check()
	}
	_ = xproto.ChangePropertyChecked(conn, xproto.PropModeReplace, window, xproto.AtomWmName,
		xproto.AtomString, 8, uint32(len(title)), []byte(title)).Check()

	gc, err := xproto.NewGcontextId(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("x11: gc id: %w", err)
	}
	if err := xproto.CreateGCChecked(conn, gc, xproto.Drawable(window),
		xproto.GcForeground, []uint32{screen.BlackPixel}).Check(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("x11: create gc: %w", err)
	}
	if err := xproto.MapWindowChecked(conn, window).Check(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("x11: map window: %w", err)
	}

	// MaximumRequestLength 以 4 位元組為單位；扣掉 PutImage 自身的標頭再留餘裕。
	maxRequest := int(setup.MaximumRequestLength)*4 - 64
	if maxRequest > 1<<20 {
		maxRequest = 1 << 20
	}

	w := &Window{
		conn: conn, window: window, gc: gc, depth: screen.RootDepth,
		sourceW: sourceW, sourceH: sourceH, scale: scale,
		image:      make([]byte, sourceW*scale*sourceH*scale*4),
		pressed:    map[xproto.Keycode]bool{},
		maxRequest: maxRequest,
	}
	w.keysyms = loadKeysyms(conn, setup)
	return w, nil
}

func deleteAtoms(conn *xgb.Conn) (xproto.Atom, xproto.Atom, bool) {
	protocols, err := xproto.InternAtom(conn, true, uint16(len("WM_PROTOCOLS")), "WM_PROTOCOLS").Reply()
	if err != nil {
		return 0, 0, false
	}
	delete, err := xproto.InternAtom(conn, false, uint16(len("WM_DELETE_WINDOW")), "WM_DELETE_WINDOW").Reply()
	if err != nil {
		return 0, 0, false
	}
	return protocols.Atom, delete.Atom, true
}

func atomBytes(atom xproto.Atom) []byte {
	value := uint32(atom)
	return []byte{byte(value), byte(value >> 8), byte(value >> 16), byte(value >> 24)}
}

// loadKeysyms 讀一次鍵盤對照表，之後用 keycode 直接查第一個 keysym。
// 失敗時回傳空表：輸入會停用，但視窗仍可正常顯示。
func loadKeysyms(conn *xgb.Conn, setup *xproto.SetupInfo) map[xproto.Keycode]uint32 {
	first := setup.MinKeycode
	count := byte(setup.MaxKeycode - setup.MinKeycode + 1)
	reply, err := xproto.GetKeyboardMapping(conn, first, count).Reply()
	if err != nil || reply.KeysymsPerKeycode == 0 {
		return map[xproto.Keycode]uint32{}
	}
	table := make(map[xproto.Keycode]uint32, int(count))
	per := int(reply.KeysymsPerKeycode)
	for index := 0; index < int(count); index++ {
		offset := index * per
		if offset >= len(reply.Keysyms) {
			break
		}
		table[first+xproto.Keycode(index)] = uint32(reply.Keysyms[offset])
	}
	return table
}

// Present 把 ARGB framebuffer 放大並送上視窗。
func (w *Window) Present(framebuffer []uint32) error {
	if len(framebuffer) < w.sourceW*w.sourceH {
		return fmt.Errorf("x11: framebuffer has %d pixels, want %d", len(framebuffer), w.sourceW*w.sourceH)
	}
	width := w.sourceW * w.scale
	for y := 0; y < w.sourceH; y++ {
		rowStart := y * w.scale * width * 4
		for x := 0; x < w.sourceW; x++ {
			pixel := framebuffer[y*w.sourceW+x]
			// X11 的 ZPixmap 在 little-endian 主機上是 BGRX 排列。
			b := byte(pixel)
			g := byte(pixel >> 8)
			r := byte(pixel >> 16)
			for sx := 0; sx < w.scale; sx++ {
				offset := rowStart + (x*w.scale+sx)*4
				w.image[offset] = b
				w.image[offset+1] = g
				w.image[offset+2] = r
				w.image[offset+3] = 0
			}
		}
		// 縱向放大直接複製第一列，避免重跑一次調色盤展開。
		source := w.image[rowStart : rowStart+width*4]
		for sy := 1; sy < w.scale; sy++ {
			copy(w.image[rowStart+sy*width*4:rowStart+(sy+1)*width*4], source)
		}
	}

	return w.flush(width, w.sourceH*w.scale)
}

// PresentRGBA 送上一張與視窗同尺寸的 RGBA 圖。覆蓋層畫在視窗的原生解析度上，
// 所以這條路徑不做放大；Present 的放大路徑保留給沒有覆蓋層的一般情況，
// 它每個像素少一次轉換。
func (w *Window) PresentRGBA(pix []byte, width, height int) error {
	if width != w.sourceW*w.scale || height != w.sourceH*w.scale {
		return fmt.Errorf("x11: image is %dx%d, window is %dx%d",
			width, height, w.sourceW*w.scale, w.sourceH*w.scale)
	}
	if len(pix) < width*height*4 {
		return fmt.Errorf("x11: image has %d bytes, want %d", len(pix), width*height*4)
	}
	// X11 的 ZPixmap 在 little-endian 主機上是 BGRX，來源是 RGBA。
	for i := 0; i < width*height; i++ {
		w.image[i*4+0] = pix[i*4+2]
		w.image[i*4+1] = pix[i*4+1]
		w.image[i*4+2] = pix[i*4+0]
		w.image[i*4+3] = 0
	}
	return w.flush(width, height)
}

// flush 把內部緩衝依 MaximumRequestLength 切條送出。
func (w *Window) flush(width, height int) error {
	rowBytes := width * 4
	rowsPerRequest := w.maxRequest / rowBytes
	if rowsPerRequest < 1 {
		rowsPerRequest = 1
	}
	for y := 0; y < height; y += rowsPerRequest {
		rows := rowsPerRequest
		if y+rows > height {
			rows = height - y
		}
		err := xproto.PutImageChecked(w.conn, xproto.ImageFormatZPixmap, xproto.Drawable(w.window), w.gc,
			uint16(width), uint16(rows), 0, int16(y), 0, w.depth,
			w.image[y*rowBytes:(y+rows)*rowBytes]).Check()
		if err != nil {
			return fmt.Errorf("x11: put image: %w", err)
		}
	}
	return nil
}

// Size 回傳視窗的像素尺寸，覆蓋層要以此建立畫布。
func (w *Window) Size() (int, int) { return w.sourceW * w.scale, w.sourceH * w.scale }

// Scale 回傳整數放大倍率。
func (w *Window) Scale() int { return w.scale }

// Poll 消化所有待處理事件並更新按鍵狀態。回傳 false 表示視窗已關閉。
func (w *Window) Poll() bool {
	for {
		event, err := w.conn.PollForEvent()
		if err != nil {
			continue
		}
		if event == nil {
			break
		}
		switch typed := event.(type) {
		case xproto.KeyPressEvent:
			w.pressed[typed.Detail] = true
		case xproto.KeyReleaseEvent:
			delete(w.pressed, typed.Detail)
		case xproto.ClientMessageEvent:
			w.closed = true
		case xproto.DestroyNotifyEvent:
			w.closed = true
		}
	}
	return !w.closed
}

// KeysymPressed 回報某個 X11 keysym 目前是否按著。
func (w *Window) KeysymPressed(keysym uint32) bool {
	for code := range w.pressed {
		if w.keysyms[code] == keysym {
			return true
		}
	}
	return false
}

func (w *Window) Close() {
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
}
