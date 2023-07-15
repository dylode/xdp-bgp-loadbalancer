package graceshut

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

type GraceShut struct {
	close chan any
	done  chan any
}

func CreateContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGQUIT)
}

func New() *GraceShut {
	return &GraceShut{
		close: make(chan any, 1),
		done:  make(chan any, 1),
	}
}

func (gc *GraceShut) Close() {
	gc.close <- 1
	close(gc.close)
}

func (gc *GraceShut) WaitForClose() {
	<-gc.close
}

func (gc *GraceShut) Done() {
	gc.done <- 1
	close(gc.done)
}

func (gc *GraceShut) WaitForDone() {
	<-gc.done
}
