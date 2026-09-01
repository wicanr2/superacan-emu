package frontend

import (
	"errors"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/wicanr2/superacan-emu/chip/umc6618"
	"github.com/wicanr2/superacan-emu/machine"
	"github.com/wicanr2/superacan-emu/presentation"
)

const DefaultFrameInstructionBound uint64 = 2_000_000

var errNoStateFile = errors.New("no save state file configured")

type keyBinding struct {
	key    ebiten.Key
	button uint16
}

var playerOneKeys = [...]keyBinding{
	{ebiten.KeyZ, machine.ButtonA}, {ebiten.KeyX, machine.ButtonB},
	{ebiten.KeyEnter, machine.ButtonStart}, {ebiten.KeyShiftRight, machine.ButtonSelect},
	{ebiten.KeyArrowUp, machine.ButtonUp}, {ebiten.KeyArrowDown, machine.ButtonDown},
	{ebiten.KeyArrowLeft, machine.ButtonLeft}, {ebiten.KeyArrowRight, machine.ButtonRight},
	{ebiten.KeyA, machine.ButtonX}, {ebiten.KeyS, machine.ButtonY},
	{ebiten.KeyQ, machine.ButtonL}, {ebiten.KeyW, machine.ButtonR},
}

// P2 的鍵位參考 Bcan.ini 的預設值（方向 R/F/D/G、按鍵 U/I/K/Y/O/P、Start=2、Select=6），
// 讓習慣 Bcan 的人不必重學。
var playerTwoKeys = [...]keyBinding{
	{ebiten.KeyU, machine.ButtonA}, {ebiten.KeyI, machine.ButtonB},
	{ebiten.KeyDigit2, machine.ButtonStart}, {ebiten.KeyDigit6, machine.ButtonSelect},
	{ebiten.KeyR, machine.ButtonUp}, {ebiten.KeyF, machine.ButtonDown},
	{ebiten.KeyD, machine.ButtonLeft}, {ebiten.KeyG, machine.ButtonRight},
	{ebiten.KeyK, machine.ButtonX}, {ebiten.KeyY, machine.ButtonY},
	{ebiten.KeyO, machine.ButtonL}, {ebiten.KeyP, machine.ButtonR},
}

func pressedButtons(bindings []keyBinding) uint16 {
	var pressed uint16
	for _, binding := range bindings {
		if ebiten.IsKeyPressed(binding.key) {
			pressed |= binding.button
		}
	}
	return pressed
}

// Game is a thin Ebitengine presentation adapter over the shared machine core.
type Game struct {
	System           *machine.System
	InstructionBound uint64
	MaxFrames        uint64
	CompletedFrames  uint64

	// SaveState 與 LoadState 由入口注入，讓呈現層不必知道檔案系統。
	// 兩者都是熱鍵路徑：失敗只回報，不中斷正在進行的遊戲。
	SaveState func() error
	LoadState func() error
	OnStatus  func(message string)

	frame       *ebiten.Image
	pixels      []byte
	savePressed bool
	loadPressed bool
}

func NewGame(system *machine.System) *Game {
	return &Game{
		System: system, InstructionBound: DefaultFrameInstructionBound,
		frame: ebiten.NewImage(umc6618.Width, umc6618.Height),
	}
}

func (g *Game) Update() error {
	g.handleStateHotkeys()
	g.System.SoundBus.SetPad(0, machine.PadState(pressedButtons(playerOneKeys[:])))
	g.System.SoundBus.SetPad(1, machine.PadState(pressedButtons(playerTwoKeys[:])))
	if _, err := g.System.RunFrame(g.InstructionBound); err != nil {
		return err
	}
	g.pixels = presentation.ARGBToRGBA(g.pixels, g.System.Bus.Video().Framebuffer())
	g.CompletedFrames++
	if g.MaxFrames != 0 && g.CompletedFrames >= g.MaxFrames {
		return ebiten.Termination
	}
	return nil
}

// handleStateHotkeys 取按下的那一瞬間，按著不放不會重複觸發。
func (g *Game) handleStateHotkeys() {
	save := ebiten.IsKeyPressed(ebiten.KeyF5)
	if save && !g.savePressed {
		g.report("save state", g.invoke(g.SaveState))
	}
	g.savePressed = save

	load := ebiten.IsKeyPressed(ebiten.KeyF7)
	if load && !g.loadPressed {
		g.report("load state", g.invoke(g.LoadState))
	}
	g.loadPressed = load
}

func (g *Game) invoke(action func() error) error {
	if action == nil {
		return errNoStateFile
	}
	return action()
}

func (g *Game) report(action string, err error) {
	if g.OnStatus == nil {
		return
	}
	if err != nil {
		g.OnStatus(action + ": " + err.Error())
		return
	}
	g.OnStatus(action + " ok")
}

func (g *Game) Draw(screen *ebiten.Image) {
	g.frame.WritePixels(g.pixels)
	screen.DrawImage(g.frame, nil)
}

func (g *Game) Layout(_, _ int) (int, int) { return umc6618.Width, umc6618.Height }
