package m68k

const (
	flagCarry    uint16 = 1 << 0
	flagOverflow uint16 = 1 << 1
	flagZero     uint16 = 1 << 2
	flagNegative uint16 = 1 << 3
	flagExtend   uint16 = 1 << 4
)

func conditionTrue(condition uint8, sr uint16) bool {
	c := sr&flagCarry != 0
	v := sr&flagOverflow != 0
	z := sr&flagZero != 0
	n := sr&flagNegative != 0

	switch condition & 0x0f {
	case 0: // T (BRA encoding)
		return true
	case 1: // F (BSR encoding; not evaluated as Bcc)
		return false
	case 2: // HI
		return !c && !z
	case 3: // LS
		return c || z
	case 4: // CC/HS
		return !c
	case 5: // CS/LO
		return c
	case 6: // NE
		return !z
	case 7: // EQ
		return z
	case 8: // VC
		return !v
	case 9: // VS
		return v
	case 10: // PL
		return !n
	case 11: // MI
		return n
	case 12: // GE
		return n == v
	case 13: // LT
		return n != v
	case 14: // GT
		return !z && n == v
	case 15: // LE
		return z || n != v
	default:
		panic("unreachable")
	}
}
