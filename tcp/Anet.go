package tcp

import (
	"net"
)

func ANetTcpServer(addr string) uintptr {
	listen, _ := net.Listen("tcp", addr)
	defer listen.Close()
	var fd uintptr
	accept, _ := listen.Accept()
	f := accept.(*net.TCPConn)
	file, _ := f.File()
	fd = file.Fd()
	return fd
}
