//go:build darwin || linux || freebsd || openbsd || netbsd

package main

import (
	"os"
	"syscall"
	"unsafe"
)

type winsize struct{ rows, cols, x, y uint16 }

func getWinsize(f *os.File) (winsize, bool) {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCGWINSZ), uintptr(unsafe.Pointer(&ws)))
	return ws, errno == 0
}

func isTerminal(f *os.File) bool {
	_, ok := getWinsize(f)
	return ok
}

func terminalWidth(f *os.File) int {
	if ws, ok := getWinsize(f); ok && ws.cols > 0 {
		return int(ws.cols)
	}
	return 0
}
