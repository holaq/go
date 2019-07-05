// +build darwin dragonfly freebsd linux nacl netbsd openbsd solaris

package signal

import (
	"bmob/library/log"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func AddKillListener(callbacks ...func()) {
	go addListeners(callbacks)
}

func addListeners(callbacks []func()) {
	sigs := make(chan os.Signal)
	defer close(sigs)

	signal.Notify(sigs,
		syscall.SIGQUIT,
		syscall.SIGTERM,
		syscall.SIGINT,
		syscall.SIGUSR1,
		syscall.SIGUSR2)

EXIT:
	for {
		select {
		case sig := <-sigs:
			switch sig {
			case syscall.SIGTERM, syscall.SIGINT, syscall.SIGQUIT:
				fmt.Println("[signals()] Interrupt...")
				break EXIT
			case syscall.SIGUSR1:
				fmt.Println("[signals()] syscall.SIGUSR1...")
			case syscall.SIGUSR2:
				fmt.Println("[signals()] syscall.SIGUSR2...")
			default:
				break EXIT
			}
		}
	}

	for _, cb := range callbacks {
		cb()
	}
	log.Info("Progress killed by user. All concluding works have be done. Exit!")
	os.Exit(0)
}
