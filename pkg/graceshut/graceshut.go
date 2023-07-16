package graceshut

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type GraceShut struct {
	close   chan any
	done    chan any
	closed  bool
	closing bool
	mut     sync.Mutex
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
	gc.mut.Lock()
	defer gc.mut.Unlock()

	if gc.closed || gc.closing {
		return
	}

	gc.closing = true
	gc.close <- 1
	close(gc.close)
}

func (gc *GraceShut) WaitForClose() chan any {
	waitChan := make(chan any, 1)
	if gc.closed {
		waitChan <- 1
		close(waitChan)
	} else {
		go func() {
			<-gc.close
			waitChan <- 1
			close(waitChan)
		}()
	}

	return waitChan
}

func (gc *GraceShut) Done() {
	if gc.closed {
		return
	}

	gc.closed = true
	gc.done <- 1
	close(gc.done)
}

func (gc *GraceShut) WaitForDone() {
	if gc.closed {
		return
	}

	<-gc.done
}
