package nodego

import (
	"bytes"
	. "github.com/yayanyang/nodego/log"
	"net"
	"time"
	//. "github.com/yayanyang/nodego/timer"
)

const (
	whoAmI Code = iota
	request
	response
	notify
)

const (
	sendCacheSize    = 56
	connRetryTimeout = time.Second * 10
	requestTimeout   = time.Second * 10
)

type whoAmIMsg struct {
	name string
}

func serviceCall(name string, dispatch chan Request, call func(request Request) interface{}) bool {
	defer func() {
		if r := recover(); r != nil {
			ERROR.Printf("dispatch request(%s) Runtime error caught: %v", name, r)
		}
	}()

	req, ok := <-dispatch

	if !ok {
		//服务被关闭
		return false
	}

	resp := call(req)

	//发送应答报文
	if req.response != nil && resp != nil {
		var buff bytes.Buffer

		req.node.encoding.Encode(&buff, resp)

		req.response <- Message{
			Code:    response,
			Id:      req.id,
			Payload: buff.Bytes()}
	}

	return true
}

//handshake 处理接入handshake请求
func (node *Node) handshake(conn net.Conn) {
	TRACE.Printf("read handshake on connection(%s)", conn.RemoteAddr())

	msg, err := node.ReadMessage(conn)

	if err != nil {
		WARN.Printf("read handshake message err,close the conn(%s)\n", conn.RemoteAddr())
		conn.Close()
		return
	}

	if msg.Code != whoAmI {
		WARN.Printf("read handshake message err,but get(%s),close the conn(%s)\n", msg.Code, conn)
		conn.Close()
		return
	}

	var whoAmI string

	if err := node.encoding.Decode(bytes.NewBuffer(msg.Payload), &whoAmI); err != nil {
		ERROR.Printf("decode whoAmI message from tcp(%s) error :%s\n", conn, err)
		conn.Close()
		return
	}

	TRACE.Printf("read handshake(%s) on connection(%s) success", whoAmI, conn.RemoteAddr())

	//通知对端代理对象，新的对端连接进入事件
	node.GetOrCreatePeer(whoAmI).InConnect(conn)
}

func (node *Node) getService(name string) chan Request {
	node.mutex.Lock()
	defer node.mutex.Unlock()

	if service, ok := node.services[name]; ok {
		return service
	} else {
		return nil
	}
}

func (node *Node) dipatchMessage(source string, msg *Message, response chan Message) {
	service := node.getService(msg.Method)

	if service == nil {
		WARN.Printf("peer(%s) try call service(%s) err : not found\n", source, msg.Method)
		return
	}

	service <- Request{msg.Id, bytes.NewBuffer(msg.Payload), node, response}
}
