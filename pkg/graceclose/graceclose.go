package graceclose

type GraceClose struct {
	close chan any
	done  chan any
}

func New() *GraceClose {
	return &GraceClose{
		close: make(chan any, 1),
		done:  make(chan any, 1),
	}
}

func (gc *GraceClose) Close() {
	gc.close <- 1
	close(gc.close)
}

func (gc *GraceClose) WaitForClose() {
	<-gc.close
}

func (gc *GraceClose) Done() {
	gc.done <- 1
	close(gc.done)
}

func (gc *GraceClose) WaitForDone() {
	<-gc.done
}
