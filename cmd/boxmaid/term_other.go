//go:build !(darwin || linux || freebsd || openbsd || netbsd)

package main

import (
	"os"
	"strconv"
)

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func terminalWidth(*os.File) int {
	n, _ := strconv.Atoi(os.Getenv("COLUMNS"))
	return n
}
