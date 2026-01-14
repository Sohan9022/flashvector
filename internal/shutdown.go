package shutdown

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func WithSignals(parent context.Context)context.Context{
	ctx,cancel := context.WithCancel(parent)
	
	sigCh := make(chan os.Signal,1)

	signal.Notify(sigCh,syscall.SIGINT,syscall.SIGTERM)

	go func(){
		<-sigCh
		cancel()
	}()

	return ctx
}

