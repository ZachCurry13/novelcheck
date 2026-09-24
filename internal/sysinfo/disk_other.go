//go:build !(linux || darwin || freebsd)

package sysinfo

// diskSpace is unknown off Linux; this only lets the tests build on a
// Windows development machine (the app itself ships as a Linux image).
func diskSpace(string) (free, total uint64) { return 0, 0 }
