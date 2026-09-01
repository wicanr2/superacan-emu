//go:build darwin

package cocoa

import (
	"fmt"
	"image"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"
)

// Cocoa 的幾何型別。欄位順序與 CoreGraphics 相同，purego 依這個順序把結構
// 放進暫存器，寫錯順序不會編譯失敗，只會畫在錯的位置。
type (
	cgFloat = float64
	cgPoint struct{ X, Y cgFloat }
	cgSize  struct{ Width, Height cgFloat }
	cgRect  struct {
		Origin cgPoint
		Size   cgSize
	}
)

// AppKit 常數。數值出自 AppKit 的標頭檔，不是猜的；它們不會隨系統版本改變。
const (
	styleMaskTitled         = 1 << 0
	styleMaskClosable       = 1 << 1
	styleMaskMiniaturizable = 1 << 2

	backingStoreBuffered = 2

	activationPolicyRegular = 0

	eventMaskAny          = ^uint(0)
	eventTypeKeyDown      = 10
	eventTypeKeyUp        = 11
	eventTypeFlagsChanged = 12
)

var (
	classNSApplication     = objc.GetClass("NSApplication")
	classNSWindow          = objc.GetClass("NSWindow")
	classNSString          = objc.GetClass("NSString")
	classNSDate            = objc.GetClass("NSDate")
	classNSBitmapImageRep  = objc.GetClass("NSBitmapImageRep")
	classNSAutoreleasePool = objc.GetClass("NSAutoreleasePool")

	selAlloc                  = objc.RegisterName("alloc")
	selInit                   = objc.RegisterName("init")
	selRelease                = objc.RegisterName("release")
	selSharedApplication      = objc.RegisterName("sharedApplication")
	selSetActivationPolicy    = objc.RegisterName("setActivationPolicy:")
	selActivateIgnoringOther  = objc.RegisterName("activateIgnoringOtherApps:")
	selFinishLaunching        = objc.RegisterName("finishLaunching")
	selNextEvent              = objc.RegisterName("nextEventMatchingMask:untilDate:inMode:dequeue:")
	selSendEvent              = objc.RegisterName("sendEvent:")
	selEventType              = objc.RegisterName("type")
	selKeyCode                = objc.RegisterName("keyCode")
	selInitWithContentRect    = objc.RegisterName("initWithContentRect:styleMask:backing:defer:")
	selSetTitle               = objc.RegisterName("setTitle:")
	selCenter                 = objc.RegisterName("center")
	selMakeKeyAndOrderFront   = objc.RegisterName("makeKeyAndOrderFront:")
	selSetDelegate            = objc.RegisterName("setDelegate:")
	selContentView            = objc.RegisterName("contentView")
	selSetWantsLayer          = objc.RegisterName("setWantsLayer:")
	selLayer                  = objc.RegisterName("layer")
	selSetContents            = objc.RegisterName("setContents:")
	selSetMagnificationFilter = objc.RegisterName("setMagnificationFilter:")
	selStringWithUTF8String   = objc.RegisterName("stringWithUTF8String:")
	selDistantPast            = objc.RegisterName("distantPast")
	selCGImage                = objc.RegisterName("CGImage")
	selInitWithBitmapData     = objc.RegisterName(
		"initWithBitmapDataPlanes:pixelsWide:pixelsHigh:bitsPerSample:samplesPerPixel:" +
			"hasAlpha:isPlanar:colorSpaceName:bytesPerRow:bitsPerPixel:")
	selClose = objc.RegisterName("close")
)

// closedWindow 讓視窗代理的 IMP 找回 Go 這一側的狀態。一個行程只開一個視窗，
// 所以用一個變數就夠；IMP 是 C 呼叫慣例，沒有地方放 Go 的閉包環境。
var closedWindow *Window

var windowDelegateClass objc.Class

func init() {
	class, err := objc.RegisterClass(
		"AcanWindowDelegate",
		objc.GetClass("NSObject"),
		nil,
		nil,
		[]objc.MethodDef{
			{
				Cmd: objc.RegisterName("windowShouldClose:"),
				Fn: objc.NewIMP(func(self objc.ID, cmd objc.SEL, sender objc.ID) bool {
					if closedWindow != nil {
						closedWindow.closed = true
					}
					return true
				}),
			},
			{
				Cmd: objc.RegisterName("windowWillClose:"),
				Fn: objc.NewIMP(func(self objc.ID, cmd objc.SEL, notification objc.ID) {
					if closedWindow != nil {
						closedWindow.closed = true
					}
				}),
			},
		},
	)
	if err != nil {
		panic(fmt.Sprintf("cocoa: register window delegate: %v", err))
	}
	windowDelegateClass = class
}

// Window 是一個 macOS 視窗。所有方法都必須在建立它的那個 OS 執行緒上呼叫：
// AppKit 只能在主執行緒操作，New 會把目前的 goroutine 綁在上面。
type Window struct {
	app     objc.ID
	window  objc.ID
	layer   objc.ID
	runMode objc.ID
	colour  objc.ID
	sourceW int
	sourceH int
	scale   int
	pixels  []byte
	pressed map[uint16]bool
	presses []uint16
	closed  bool
}

func nsString(text string) objc.ID {
	bytes := append([]byte(text), 0)
	return objc.ID(classNSString).Send(selStringWithUTF8String, unsafe.Pointer(&bytes[0]))
}

// New 建立並顯示一個 sourceW×sourceH 以 scale 倍放大的視窗。
//
// 呼叫端必須在 main 的 goroutine 上呼叫，而且之後的 Poll／Present 也要在同一個
// goroutine：AppKit 的所有操作都限定主執行緒，違反時的症狀是隨機當掉而不是錯誤。
func New(title string, sourceW, sourceH, scale int) (*Window, error) {
	if scale < 1 {
		return nil, fmt.Errorf("cocoa: scale must be at least 1")
	}
	runtime.LockOSThread()

	app := objc.ID(classNSApplication).Send(selSharedApplication)
	app.Send(selSetActivationPolicy, activationPolicyRegular)
	app.Send(selFinishLaunching)
	app.Send(selActivateIgnoringOther, true)

	rect := cgRect{Size: cgSize{Width: cgFloat(sourceW * scale), Height: cgFloat(sourceH * scale)}}
	style := uint(styleMaskTitled | styleMaskClosable | styleMaskMiniaturizable)
	window := objc.ID(classNSWindow).Send(selAlloc).Send(
		selInitWithContentRect, rect, style, uint(backingStoreBuffered), false)
	if window == 0 {
		return nil, fmt.Errorf("cocoa: create window")
	}
	window.Send(selSetTitle, nsString(title))
	window.Send(selCenter)

	view := window.Send(selContentView)
	view.Send(selSetWantsLayer, true)
	layer := view.Send(selLayer)
	if layer == 0 {
		return nil, fmt.Errorf("cocoa: content view has no layer")
	}
	// 最近鄰放大：硬體輸出是點陣，內插只會製造原本不存在的中間色。
	layer.Send(selSetMagnificationFilter, nsString("nearest"))

	w := &Window{
		app: app, window: window, layer: layer,
		runMode: nsString("kCFRunLoopDefaultMode"),
		colour:  nsString("NSDeviceRGBColorSpace"),
		sourceW: sourceW, sourceH: sourceH, scale: scale,
		pressed: map[uint16]bool{},
	}
	closedWindow = w

	delegate := objc.ID(windowDelegateClass).Send(selAlloc).Send(selInit)
	window.Send(selSetDelegate, delegate)
	window.Send(selMakeKeyAndOrderFront, objc.ID(0))
	return w, nil
}

// Size 回傳視窗的像素尺寸。
func (w *Window) Size() (int, int) { return w.sourceW * w.scale, w.sourceH * w.scale }

// Scale 回傳整數放大倍率。
func (w *Window) Scale() int { return w.scale }

// Poll 消化所有待處理事件並更新按鍵狀態。回傳 false 表示視窗已關閉。
func (w *Window) Poll() bool {
	pool := objc.ID(classNSAutoreleasePool).Send(selAlloc).Send(selInit)
	defer pool.Send(selRelease)

	distantPast := objc.ID(classNSDate).Send(selDistantPast)
	for {
		event := w.app.Send(selNextEvent, eventMaskAny, distantPast, w.runMode, true)
		if event == 0 {
			break
		}
		switch uint(event.Send(selEventType)) {
		case eventTypeKeyDown:
			code := uint16(event.Send(selKeyCode))
			if !w.pressed[code] && len(w.presses) < 64 {
				// 綁定畫面要知道「剛剛按下的是哪一個鍵」，而不是「這個鍵現在
				// 有沒有被按著」。這條佇列有上限，避免沒人取用時無限成長。
				w.presses = append(w.presses, code)
			}
			w.pressed[code] = true
		case eventTypeKeyUp:
			delete(w.pressed, uint16(event.Send(selKeyCode)))
		}
		w.app.Send(selSendEvent, event)
	}
	return !w.closed
}

// KeysymPressed 回報一個虛擬鍵碼目前是否被按著。名稱與 X11 前端一致，
// 讓兩邊的入口程式長得一樣。
func (w *Window) KeysymPressed(code uint32) bool { return w.pressed[uint16(code)] }

// TakeKeyPresses 取走並清空自上次呼叫以來按下的鍵碼。
func (w *Window) TakeKeyPresses() []uint32 {
	if len(w.presses) == 0 {
		return nil
	}
	out := make([]uint32, len(w.presses))
	for index, code := range w.presses {
		out[index] = uint32(code)
	}
	w.presses = w.presses[:0]
	return out
}

// Present 把一張 RGBA 圖送上視窗。圖可以是 320×240 的原始畫面，也可以是與視窗
// 同尺寸的合成畫面；放大交給 layer 以最近鄰完成。
func (w *Window) Present(frame *image.RGBA) error {
	if frame == nil {
		return fmt.Errorf("cocoa: nil frame")
	}
	bounds := frame.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return fmt.Errorf("cocoa: frame is %dx%d", width, height)
	}
	// 自己保留一份像素：NSBitmapImageRep 不複製 planes，來源緩衝在下一幀被
	// 覆寫時畫面就會撕裂。
	need := width * height * 4
	if len(w.pixels) != need {
		w.pixels = make([]byte, need)
	}
	if frame.Stride == width*4 {
		copy(w.pixels, frame.Pix[:need])
	} else {
		for y := 0; y < height; y++ {
			copy(w.pixels[y*width*4:(y+1)*width*4], frame.Pix[y*frame.Stride:y*frame.Stride+width*4])
		}
	}

	pool := objc.ID(classNSAutoreleasePool).Send(selAlloc).Send(selInit)
	defer pool.Send(selRelease)

	plane := &w.pixels[0]
	// hasAlpha:false 配 bitsPerPixel:32：第四個位元組當成填充，因此不必回答
	// 「alpha 是不是預乘」這個問題——我們的畫面本來就不透明。
	rep := objc.ID(classNSBitmapImageRep).Send(selAlloc).Send(
		selInitWithBitmapData,
		unsafe.Pointer(&plane),
		uint(width), uint(height),
		uint(8), uint(3),
		false, false,
		w.colour,
		uint(width*4), uint(32),
	)
	if rep == 0 {
		return fmt.Errorf("cocoa: create bitmap representation")
	}
	defer rep.Send(selRelease)

	cgImage := rep.Send(selCGImage)
	if cgImage == 0 {
		return fmt.Errorf("cocoa: bitmap representation has no CGImage")
	}
	w.layer.Send(selSetContents, cgImage)
	return nil
}

// Close 關閉視窗。
func (w *Window) Close() {
	if w.window != 0 {
		w.window.Send(selClose)
		w.window = 0
	}
	if closedWindow == w {
		closedWindow = nil
	}
}
