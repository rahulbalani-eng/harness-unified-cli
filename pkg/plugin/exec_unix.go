// Copyright © 2026 Harness Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package plugin

import (
	"os"
	"syscall"
)

// Exec replaces the current process with binPath via syscall.Exec (execve).
func Exec(binPath string, args []string) error {
	argv := append([]string{binPath}, args...)
	return syscall.Exec(binPath, argv, os.Environ())
}
