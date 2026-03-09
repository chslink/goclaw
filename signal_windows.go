//go:build windows

package main

func setupStackDumpSignal() {
	// Windows does not support SIGUSR1/SIGQUIT
	// Stack dump signal handling is disabled on Windows
}
