package runtime

import "unsafe"

//uart registers
const UARTBASE uint32 = 0x9000000
const UARTDR uint32 = 0x0 + UARTBASE
const UARTFR uint32 = 0x18 + UARTBASE
const TXFF uint32 = 0x1 << 5

//go:nosplit
//go:nobarrierec
func write_virtv7(buf uintptr, count uint32) uint32 {
	uartdr := (*byte)(unsafe.Pointer(uintptr(UARTDR)))
	uartfr := (*uint32)(unsafe.Pointer(uintptr(UARTFR)))
	for i := uint32(0); i < count; i++ {
		//get character
		c := ((*byte)(unsafe.Pointer(buf + uintptr(i))))
		//check for space in FIFO
		for (*uartfr & TXFF) > 0 {
		}
		//throw it in
		*uartdr = *c
	}
	return count
}
