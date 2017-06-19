// Copyright 2017 Yanni Coroneos. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build !android

package runtime

import "unsafe"

func writeErr(b []byte) {
	write(2, unsafe.Pointer(&b[0]), int32(len(b)))
	//	if Armhackmode > 0 {
	//
	//		write_uart(b)
	//
	//	} else {
	//		write(2, unsafe.Pointer(&b[0]), int32(len(b)))
	//	}
}

//printing
const UART1_UTXD uint32 = 0x02020040
const UART1_UTS uint32 = 0x020200B4

//go:nosplit
func uart_putc(c byte) {
	*(*byte)(unsafe.Pointer(uintptr(UART1_UTXD))) = c
	for (*(*uint32)(unsafe.Pointer(uintptr(UART1_UTS))) & (0x1 << 6)) <= 0 {
	}
}

const (
	READ_MASK = 0xFF
	RRDY      = 1 << 9
)

type UART_regs struct {
	urxd  uint32
	_     [15]uint32 //thanks freescale
	utxd  uint32
	_     [15]uint32 //thanks freescale
	ucr1  uint32
	ucr2  uint32
	ucr3  uint32
	ucr4  uint32
	ufcr  uint32
	usr1  uint32
	usr2  uint32
	uesc  uint32
	utim  uint32
	ubir  uint32
	ubmr  uint32
	ubrc  uint32
	onems uint32
	uts   uint32
	umcr  uint32
}

type UART struct {
	regs *UART_regs
}

var WB_DEFAULT_UART = UART{((*UART_regs)(unsafe.Pointer(uintptr(0x2020000))))}

//blocking read
//go:nosplit
func (u *UART) getchar() byte {
	for (u.regs.usr1 & RRDY) == 0 {
	}
	return byte(u.regs.urxd & READ_MASK)
}

//go:nosplit
func (u *UART) Read(n int) []byte {
	output := make([]byte, n)
	for i := 0; i < n; i++ {
		output[i] = u.getchar()
	}
	return output
}

//go:nosplit
//go:nobarrierec
func write_uart(b []byte) {
	var i = 0
	for i = 0; i < len(b); i++ {
		uart_putc(b[i])
	}
}

//go:nosplit
//go:nobarrierec
func write_uart_unsafe(buf uintptr, count uint32) uint32 {
	for i := uint32(0); i < count; i++ {
		c := ((*byte)(unsafe.Pointer(buf + uintptr(i))))
		uart_putc(*c)
	}
	return count
}
