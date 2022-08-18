package tcp

import (
	"golang.org/x/sys/unix"
	"log"
)

type AeApiState struct {
	events []unix.EpollEvent
	epfd   int
}

func AeApiCreate(eventLoop AeEventLoop) int {
	state := AeApiState{
		events: nil,
		epfd:   0,
	}
	state.events = make([]unix.EpollEvent, eventLoop.setsize)
	state.epfd, _ = unix.EpollCreate1(1024)
	eventLoop.apidata = &state
	return 0
}

func AeApiResize(eventLoop *AeEventLoop, setsize int) int {
	var state *AeApiState
	state = eventLoop.apidata
	state.events = make([]unix.EpollEvent, setsize)
	return 0
}

func AeApiAddEvent(eventLoop *AeEventLoop, fd int, mask int) int {
	state := eventLoop.apidata
	ee := unix.EpollEvent{
		Events: 0,
		Fd:     0,
		Pad:    0,
	}
	var op int
	if eventLoop.events[fd].mask == AE_NONE {
		op = unix.EPOLL_CTL_ADD
	} else {
		op = unix.EPOLL_CTL_MOD
	}
	mask |= eventLoop.events[fd].mask
	if mask&AE_READABLE == 1 {
		ee.Events |= unix.EPOLLIN
	}
	if mask&AE_WRITABLE == 1 {
		ee.Events |= unix.EPOLLOUT
	}
	ee.Fd = int32(fd)
	e1 := unix.EpollCtl(state.epfd, op, fd, &ee)
	if e1 != nil {
		return -1
	}
	return 0
}

func AeApiDelEvent(eventLoop *AeEventLoop, fd int, delmask int) {
	state := eventLoop.apidata
	ee := unix.EpollEvent{
		Events: 0,
		Fd:     0,
		Pad:    0,
	}
	mask := eventLoop.events[fd].mask & (^delmask)
	if mask&AE_READABLE == 1 {
		ee.Events |= unix.EPOLLIN
	}
	if mask&AE_WRITABLE == 1 {
		ee.Events |= unix.EPOLLOUT
	}
	ee.Fd = int32(fd)
	if mask != AE_NONE {
		unix.EpollCtl(state.epfd, unix.EPOLL_CTL_MOD, fd, &ee)
	} else {
		unix.EpollCtl(state.epfd, unix.EPOLL_CTL_DEL, fd, &ee)
	}
}

func AeApiPoll(eventLoop *AeEventLoop, tvp *unix.Timeval) int {
	state := eventLoop.apidata
	retval, numevents := 0, 0
	tmp := tvp.Sec*1000 + (tvp.Usec+999)/1000
	if tvp == nil {
		tmp = -1
	}
	retval, _ = unix.EpollWait(state.epfd, state.events, int(tmp))
	if retval > 0 {
		numevents = retval
		for j := 0; j < numevents; j++ {
			mask := 0
			e := state.events[j]
			if e.Events&unix.EPOLLIN != 0 {
				mask |= AE_READABLE
			}
			if e.Events&unix.EPOLLOUT != 0 {
				mask |= AE_WRITABLE
			}
			if e.Events&unix.EPOLLERR != 0 {
				mask |= AE_WRITABLE | AE_READABLE
			}
			if e.Events&unix.EPOLLHUP != 0 {
				mask |= AE_WRITABLE | AE_READABLE
			}
			eventLoop.fired[j].fd = int(e.Fd)
			eventLoop.fired[j].mask = mask
		}

	} else if retval == -1 {
		log.Panic("aeapipoll epoll wait err")

	}
	return numevents
}
func AeApiName() string {
	return "epoll"
}
