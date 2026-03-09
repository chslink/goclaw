//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func setupStackDumpSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1, syscall.SIGQUIT)

	go func() {
		for range ch {
			dumpGoroutineStacks()
		}
	}()
}
