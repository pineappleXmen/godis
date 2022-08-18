package tcp

import (
	"golang.org/x/sys/unix"
	"redis-1.0/util"
)

//type AeFileProc interface {
//	aeFileProc(eventLoop *AeEventLoop, fd int, clientData *interface{}, mask int)
//}
//type AeTimeProc interface {
//	aeTimeProc(eventLoop *AeEventLoop, id int64, mask, clientData *interface{})
//}
//type AeEventFinalizerProc interface {
//	aeEventFinalizerProc(eventLoop *AeEventLoop, clientData *interface{})
//}
//type AeBeforeSleepProc interface {
//	aeTimeProc(eventLoop *AeEventLoop)
//}

const (
	AE_OK                = 0
	AE_ERR               = 1
	AE_NONE              = 0
	AE_READABLE          = 1
	AE_WRITABLE          = 2
	AE_BARRIER           = 4
	AE_FILE_EVENTS       = 1 << 0
	AE_TIME_EVENTS       = 1 << 1
	AE_ALL_EVENTS        = AE_FILE_EVENTS | AE_TIME_EVENTS
	AE_DONT_WAIT         = 1 << 2
	AE_CALL_BEFORE_SLEEP = 1 << 3
	AE_CALL_AFTER_SLEEP  = 1 << 4
	AE_NOMORE            = -1
	AE_DELETED_EVENT_ID  = -1
)

type AeEventLoop struct {
	maxfd           int
	setsize         int
	timeEventNextId int64
	events          []*AeFileEvent
	fired           []*AeFiredEvent
	timeEvent       *AeTimeEvent
	stop            int
	apidata         *AeApiState
	beforesleep     func(eventLoop *AeEventLoop)
	aftersleep      func(eventLoop *AeEventLoop)
	flag            int
}

type AeFileEvent struct {
	mask       int
	rfileProc  func(eventLoop *AeEventLoop, fd int, clientData interface{}, mask int)
	wfileProc  func(eventLoop *AeEventLoop, fd int, clientData interface{}, mask int)
	clientData interface{}
}
type aeFileProc struct {
	eventLoop *AeEventLoop
	fd        int
}

type AeTimeEvent struct {
	id            int64
	TimeProc      func(eventLoop *AeEventLoop, id int64, clientData *interface{}) int64
	FinalizerProc func(eventLoop *AeEventLoop, clientData *interface{})
	clinetData    *interface{}
	prev          *AeTimeEvent
	next          *AeTimeEvent
	refcount      int
	when          int64
}

type AeFiredEvent struct {
	fd   int
	mask int
}

func AeCreateEventLoop(setsize int) *AeEventLoop {
	e := &AeEventLoop{
		maxfd:           -1,
		setsize:         setsize,
		timeEventNextId: 0,
		events:          make([]*AeFileEvent, setsize),
		fired:           make([]*AeFiredEvent, setsize),
		timeEvent:       &AeTimeEvent{},
		stop:            0,
		apidata:         nil,
		beforesleep:     nil,
		aftersleep:      nil,
		flag:            0,
	}
	for i := 0; i < setsize; i++ {
		e.events[i].mask = AE_NONE
	}
	return e
}
func AeGetSetSize(eventLoop *AeEventLoop) int {
	return eventLoop.setsize
}

func AeSetDontWait(eventLoop *AeEventLoop, noWait int) {
	if noWait == 1 {
		eventLoop.flag |= AE_DONT_WAIT
	} else {
		eventLoop.flag &= (^AE_DONT_WAIT)
	}
}
func AeResizeSetSize(eventLoop *AeEventLoop, setsize int) int {
	if setsize == eventLoop.setsize {
		return AE_OK
	}
	if eventLoop.maxfd >= setsize {
		return AE_ERR
	}
	if AeApiResize(eventLoop, setsize) == -1 {
		return AE_ERR
	}
	eventLoop.events = make([]*AeFileEvent, setsize)
	eventLoop.fired = make([]*AeFiredEvent, setsize)
	eventLoop.setsize = setsize
	for i := eventLoop.maxfd; i < setsize; i++ {
		eventLoop.events[i].mask = AE_NONE
	}
	return AE_OK
}

func AeDeleteEventLoop(eventLoop *AeEventLoop) {
	eventLoop.events = nil
	eventLoop.fired = nil
	eventLoop = nil
}
func AeStop(eventLoop *AeEventLoop) {
	eventLoop.stop = 1
}

func AeCreateFileEvent(eventLoop *AeEventLoop, fd int, mask int, proc func(eventLoop *AeEventLoop, fd int, clientData interface{}, mask int), clientdata *interface{}) int {
	if fd >= eventLoop.setsize {
		return AE_ERR
	}
	fe := eventLoop.events[fd]
	if AeApiAddEvent(eventLoop, fd, mask) == -1 {
		return AE_ERR
	}
	fe.mask |= mask
	if mask&AE_READABLE != 0 {
		fe.rfileProc = proc
	}
	if mask&AE_WRITABLE != 0 {
		fe.wfileProc = proc
	}
	fe.clientData = clientdata
	if fd > eventLoop.maxfd {
		eventLoop.maxfd = fd
	}
	return AE_OK
}
func AeDeleteFileEvent(eventLoop *AeEventLoop, fd int, mask int) int {
	if fd >= eventLoop.setsize {
		return AE_ERR
	}
	fe := eventLoop.events[fd]
	if AeApiAddEvent(eventLoop, fd, mask) == -1 {
		return AE_ERR
	}
	fe.mask |= mask
	if fd > eventLoop.maxfd {
		eventLoop.maxfd = fd
	}
	return AE_OK
}

func AeGetFileClientData(eventLoop *AeEventLoop, fd int) interface{} {
	if fd >= eventLoop.setsize {
		return nil
	}
	fe := eventLoop.events[fd]
	if fe.mask == AE_NONE {
		return nil
	}
	return fe.clientData
}

func AeGetFileEvents(eventLoop *AeEventLoop, fd int) int {
	if fd >= eventLoop.setsize {
		return 0
	}
	fe := eventLoop.events[fd]
	return fe.mask
}

func AeCreateTimeEvent(eventLoop *AeEventLoop, millisecond int64, proc func(eventLoop *AeEventLoop, id int64,
	clientData *interface{}) int64, clientData interface{}, finalizerProc func(eventLoop *AeEventLoop, clientData *interface{})) int64 {
	eventLoop.timeEventNextId++
	id := eventLoop.timeEventNextId
	te := AeTimeEvent{}
	te.id = id
	te.when = util.GetMonotonicUs() + 1000*millisecond
	te.TimeProc = proc
	te.FinalizerProc = finalizerProc
	te.prev = nil
	te.next = eventLoop.timeEvent
	te.refcount = 0
	if te.next != nil {
		te.next.prev = &te
		eventLoop.timeEvent = &te
	}
	return id
}

func AeDeleteTimeEvent(eventLoop *AeEventLoop, id int64) int {
	te := eventLoop.timeEvent
	for te != nil {
		if te.id == id {
			te.id = AE_DELETED_EVENT_ID
			return AE_OK
		}
		te = te.next

	}
	return AE_ERR
}

func usUntilEarliestTimer(eventLoop *AeEventLoop) int {
	te := eventLoop.timeEvent
	if te == nil {
		return -1
	}
	earliest := &AeTimeEvent{}
	for te != nil {
		if earliest == nil || te.when < earliest.when {
			earliest = te
			te = te.next
		}
	}
	now := util.GetMonotonicUs()
	if now >= earliest.when {
		return 0
	} else {
		return int(earliest.when - now)
	}

}

func ProcessTimeEvents(eventLoop *AeEventLoop) int {
	processed := 0
	te := eventLoop.timeEvent
	//maxid := eventLoop.timeEventNextId - 1
	now := util.GetMonotonicUs()
	for te != nil {
		if te.id == AE_DELETED_EVENT_ID {
			next := te.next
			if te.refcount != 0 {
				te = next
				continue
			}
			if te.prev != nil {
				te.prev.next = te.next
			} else {
				eventLoop.timeEvent = te.next
			}
			if te.next != nil {
				te.next.prev = te.prev
			}
			if te.FinalizerProc != nil {
				te.FinalizerProc(eventLoop, te.clinetData)
				now = util.GetMonotonicUs()
			}
			te = next
			continue
		}
		if te.when <= now {
			id := te.id
			te.refcount++
			retval := te.TimeProc(eventLoop, id, te.clinetData)
			te.refcount--
			processed++
			now = util.GetMonotonicUs()
			if retval != AE_NOMORE {
				te.when = now + retval*1000
			} else {
				te.id = AE_DELETED_EVENT_ID
			}
		}
		te = te.next
	}
	return processed
}

func AeSetBeforeSleepProc(eventLoop *AeEventLoop, beforeSleep func(eventLoop *AeEventLoop)) {
	eventLoop.beforesleep = beforeSleep
}
func AeSetAfterSleepProc(eventLoop *AeEventLoop, afterSleep func(eventLoop *AeEventLoop)) {
	eventLoop.aftersleep = afterSleep
}
func AeMain(eventLoop *AeEventLoop) {
	eventLoop.stop = 0
	for eventLoop.stop == 0 {
		aeProcessEvents(eventLoop, AE_ALL_EVENTS|AE_CALL_BEFORE_SLEEP|AE_CALL_AFTER_SLEEP)
	}
}

func aeProcessEvents(eventLoop *AeEventLoop, flags int) int {
	processed := 0
	if (flags&AE_TIME_EVENTS) != 0 && (flags&AE_FILE_EVENTS) != 0 {
		return 0
	}
	if eventLoop.maxfd != -1 || ((flags&AE_TIME_EVENTS) == 0 && flags&AE_DONT_WAIT != 0) {
		tv := unix.Timeval{}
		tvp := &unix.Timeval{}
		usUntilTimer := -1
		if flags&AE_TIME_EVENTS == 0 && ((flags & AE_DONT_WAIT) != 0) {
			usUntilTimer = usutilearliestimer(eventLoop)
		}
		if usUntilTimer >= 0 {
			tv.Sec = int64(usUntilTimer / 1000000)
			tv.Usec = int64(usUntilTimer % 1000000)
			tvp = &tv
		} else {
			if (flags & AE_DONT_WAIT) != 0 {
				tv.Sec = 0
				tv.Usec = 0
			} else {
				tvp = nil
			}
		}
		if eventLoop.beforesleep != nil && (flags&AE_CALL_BEFORE_SLEEP) != 0 {
			eventLoop.beforesleep(eventLoop)
		}

		numevents := AeApiPoll(eventLoop, tvp)
		if eventLoop.aftersleep != nil && (flags&AE_CALL_AFTER_SLEEP) != 0 {
			eventLoop.aftersleep(eventLoop)
		}
		for j := 0; j < numevents; j++ {
			fd := eventLoop.fired[j].fd
			fe := eventLoop.events[fd]
			mask := eventLoop.fired[j].mask
			fired := 0
			invert := fe.mask & AE_BARRIER
			if invert == 0 && (fe.mask&mask&AE_READABLE) != 0 {
				fe.rfileProc(eventLoop, fd, fe.clientData, mask)
				fired++
				fe = eventLoop.events[fd]
			}
			if fe.mask&mask&AE_WRITABLE != 0 {
				if fired == 0 || &fe.wfileProc != &fe.rfileProc {
					fe.rfileProc(eventLoop, fd, fe.clientData, mask)
					fired++
				}
			}
			if invert != 0 {
				fe = eventLoop.events[fd]
				if fe.mask&mask&AE_READABLE != 0 && (fired == 0 || &fe.wfileProc != &fe.rfileProc) {
					fe.rfileProc(eventLoop, fd, fe.clientData, mask)
					fired++
				}
			}
			processed++
		}

	}
	if flags&AE_TIME_EVENTS != 0 {
		processed += processTimeEvents(eventLoop)
	}
	return processed
}

func AeWait(fd int, mask int, milli int64) int {
	retmask := 0
	pfd := []unix.PollFd{}
	pfd[fd].Fd = int32(fd)
	if mask&AE_READABLE != 0 {
		pfd[fd].Events |= unix.POLLIN
	}
	if mask&AE_WRITABLE != 0 {
		pfd[fd].Events |= unix.POLLOUT
	}
	poll, _ := unix.Poll(pfd, 1)
	retval := poll
	if poll == 1 {
		if pfd[fd].Revents&unix.POLLIN != 0 {
			retmask |= AE_READABLE
		}
		if pfd[fd].Revents&unix.POLLOUT != 0 {
			retmask |= AE_WRITABLE
		}
		if pfd[fd].Revents&unix.POLLERR != 0 {
			retmask |= AE_WRITABLE
		}
		if pfd[fd].Revents&unix.POLLHUP != 0 {
			retmask |= AE_WRITABLE
		}
		return retmask
	} else {
		return retval
	}
}

func processTimeEvents(eventLoop *AeEventLoop) int {
	return 0
}

func usutilearliestimer(loop *AeEventLoop) int {
	return 0
}
