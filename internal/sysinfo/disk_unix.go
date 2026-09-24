//go:build linux || darwin || freebsd

package sysinfo

import "syscall"

// diskSpace returns the free and total bytes of the filesystem holding dir.
func diskSpace(dir string) (free, total uint64) {
	var st syscall.Statfs_t
	if syscall.Statfs(dir, &st) != nil {
		return 0, 0
	}
	return st.Bavail * uint64(st.Bsize), st.Blocks * uint64(st.Bsize)
}
