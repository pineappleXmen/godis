package server

import (
	"log"
	"redis-1.0/tcp"
)

const (
	C_OK  = 0
	C_ERR = -1
)

type RedisServer struct {
	El            *tcp.AeEventLoop
	Port          int
	BindAddr      string
	BindAddrCount int
	MaxClients    int64
	Ipfd          socketFds
}
type socketFds struct {
	fd    [16]int
	count int
}

var server RedisServer

func InitServer() {
	server.El = tcp.AeCreateEventLoop(32 + 96)
	if server.El == nil {
		log.Print("fail to init the eventloop")
	}
	listenToPort(4379, &server.Ipfd)
	CreateSocketAcceptHandler(&server.Ipfd, acceptTcpHandler)
	tcp.AeSetBeforeSleepProc(server.El, beforeSleep)
	tcp.AeSetAfterSleepProc(server.El, afterSleep)
	tcp.AeMain(server.El)
}

func listenToPort(port int, sfd *socketFds) int {
	bindaddr := server.BindAddr
	if server.BindAddrCount == 0 {
		return C_OK
	}
	sfd.fd[sfd.count] = int(tcp.ANetTcpServer(bindaddr))
	sfd.count++
	return C_OK
}

func CreateSocketAcceptHandler(sfd *socketFds, accept_handler func(eventLoop *tcp.AeEventLoop, fd int, clientData *interface{}, mask int)) int {
	for j := 0; j < sfd.count; j++ {
		if tcp.AeCreateFileEvent(server.El, sfd.fd[j], tcp.AE_READABLE, accept_handler, nil) == tcp.AE_ERR {
			for j = j - 1; j >= 0; j-- {
				tcp.AeDeleteFileEvent(server.El, sfd.fd[j], tcp.AE_READABLE)
				return C_ERR
			}
		}
	}
	return C_OK
}
func beforeSleep(eventLoop *tcp.AeEventLoop) {

}
func afterSleep(eventLoop *tcp.AeEventLoop) {

}

func acceptTcpHandler(el *tcp.AeEventLoop, fd int, privdata *interface{}, mask int) {

}
