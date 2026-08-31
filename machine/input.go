package machine

// Controller bits use the physical 16-bit order. Internal pad state is active
// low; callers normally combine Button* masks as pressed controls and pass
// them through PadState.
const (
	ButtonA      uint16 = 1 << 15
	ButtonB      uint16 = 1 << 14
	ButtonStart  uint16 = 1 << 13
	ButtonSelect uint16 = 1 << 12
	ButtonUp     uint16 = 1 << 11
	ButtonDown   uint16 = 1 << 10
	ButtonLeft   uint16 = 1 << 9
	ButtonRight  uint16 = 1 << 8
	ButtonX      uint16 = 1 << 7
	ButtonY      uint16 = 1 << 6
	ButtonL      uint16 = 1 << 5
	ButtonR      uint16 = 1 << 4
	PadReleased  uint16 = 0xffff
)

func PadState(pressed uint16) uint16 { return ^pressed }
