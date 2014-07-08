package nodego

import(
	"io"
	"net"
	"time"
	"sync"
	"bytes"
	"errors"
	"encoding/binary"
	. "github.com/yayanyang/nodego/log"
	//. "github.com/yayanyang/nodego/timer"
)

type id uint32

type code uint8

const (
	whoAmI code = iota
	request
	response
	notify
)

var (
	RequestTimeout = errors.New("request timeout")
	ServiceNameError = errors.New("serivce name error")
)

const (
	sendCacheSize = 56
	connRetryTimeout = time.Second * 10
)

type Message struct {
	Code 		code
	Id 			id
	Method      string
	Payload 	[]byte
}

type whoAmIMsg struct {
	name		string
}

type Encoding interface {
	Decode(reader io.Reader, e interface{}) error
	Encode(writer io.Writer, e interface{}) error
}

type Node struct {
	name 		string
	services    map[string] chan Request
	peers 		map[string] Peer
	listener 	net.Listener
	encoding    Encoding
	mutex 		sync.Mutex
}

type Response struct {
	Error			error
	reader 			io.Reader 
	node 			*Node
}

func (response * Response) Get(e interface{}) error{
	return response.node.encoding.Decode(response.reader,e)
}

type Future <- chan Response

type Request struct {
	id 				id
	reader 			io.Reader
	node 			*Node
	response		chan Message
}

func (request * Request) Get(e interface{}) error{
	return request.node.encoding.Decode(request.reader,e)
}

func New(laddr string) (*Node, error) {
	//创建监听socket
	var (
		listener net.Listener
		err error
	)
	if listener, err = net.Listen("tcp",laddr); err != nil {
		ERROR.Printf("create tcp(%s) listen socket error :%s\n",laddr,err)
		return nil, err
	}

	return &Node{
		name : laddr,
		services:make(map[string] chan Request),
		peers:make(map[string] Peer),
		listener:listener,
		encoding : &gobEncoding{}}, nil
}

func (node *Node) Run(){
	for {
		conn, err := node.listener.Accept()

		if err != nil {
			ERROR.Printf("call tcp(%s) accept error :%s\n",node.listener,err)
			continue
		}
		TRACE.Printf("accept read connection(%s)",conn.RemoteAddr())
		go node.handshake(conn)
	} 
}

func serviceCall(name string, dispatch chan Request, call func(request Request) interface{}) bool {
	defer func() {
		if r := recover(); r != nil {
            ERROR.Printf("dispatch request(%s) Runtime error caught: %v",name, r)
        }
	}()

	req ,ok := <- dispatch

	if !ok {
		//服务被关闭
		return false
	} 

	resp := call(req)

	//发送应答报文
	if req.response != nil && resp != nil {
		var buff bytes.Buffer

		req.node.encoding.Encode(&buff,resp)

		req.response <- Message{
			Code : response,
			Id : req.id,
			Payload : buff.Bytes()}
	}

	return true
}

func (node *Node) Register(name string, call func(request Request) interface{} ) error {

	node.mutex.Lock()

	defer node.mutex.Unlock()

	if _, ok := node.services[name]; ok {
		return ServiceNameError
	}

	serviceChan := make(chan Request,sendCacheSize)

	go func(){
		for {
			if !serviceCall(name, serviceChan, call) {
				break
			}
		}
	}()

	node.services[name] = serviceChan

	return nil
}

func (node *Node) Call(target string, method string, data interface{}) (Future, error) {
	peer := node.GetOrCreatePeer(target)

	return peer.SendRequest(method, data)
}

func (node *Node) Notify(target string, method string, data interface{}) error {
	peer := node.GetOrCreatePeer(target)

	return peer.Post(method, data)
}

func (node *Node) CallWithTimeout(target string, method string, timeout time.Duration, data interface{}) (Future, error) {
	peer := node.GetOrCreatePeer(target)

	return peer.SendRequestWithTimeout(method, timeout, data)
}

func (node *Node) ReadMessage( conn net.Conn ) (*Message,error){
	header := make([]byte,2)
	//读取消息头，两个字节大端序。表示消息体长度
	if _, err := io.ReadFull(conn,header); err != nil {
		ERROR.Printf("read message length from tcp(%s) error :%s\n",conn,err)
		return nil, err
	}

	//转换字节序,最长65535
	//TODO:限制最长报文到一个合理范围
	length := binary.BigEndian.Uint16(header)

	body := make([]byte,length)

	if readlen, err := io.ReadFull(conn,body); err != nil {
		ERROR.Printf("read message body(%d:%d) from tcp(%s) error :%s\n",length,readlen,conn.RemoteAddr(),err)
		return nil, err
	}

	var msg Message



	if err := node.encoding.Decode(bytes.NewBuffer(body),&msg); err != nil {
		ERROR.Printf("decode message body(%d) from tcp(%s) error :%s,%v",length,conn.RemoteAddr(),err,body)
		return nil, err
	}

	return &msg , nil
}

func (node *Node) GetOrCreatePeer(name string) Peer {
	node.mutex.Lock()
	defer node.mutex.Unlock()

	if peer , ok := node.peers[name] ;  ok {
		//查询到已经创建对端代理对象，直接返回该对象
		return peer
	} else {
		//创建新的对端代理对象
		peer = createPeer(node,name)
		node.peers[name] = peer
		return peer
	}
}

//handshake 处理接入handshake请求
func (node *Node) handshake( conn net.Conn ) {
	TRACE.Printf("read handshake on connection(%s)",conn.RemoteAddr())

	msg , err := node.ReadMessage(conn);

	if err != nil {
		WARN.Printf("read handshake message err,close the conn(%s)\n",conn.RemoteAddr())
		conn.Close()
		return
	}

	if msg.Code != whoAmI {
		WARN.Printf("read handshake message err,but get(%s),close the conn(%s)\n",msg.Code,conn)
		conn.Close()
		return
	}

	var whoAmI string

	if err := node.encoding.Decode(bytes.NewBuffer(msg.Payload),&whoAmI); err != nil {
		ERROR.Printf("decode whoAmI message from tcp(%s) error :%s\n",conn,err)
		conn.Close()
		return
	}

	TRACE.Printf("read handshake(%s) on connection(%s) success",whoAmI,conn.RemoteAddr())

	//通知对端代理对象，新的对端连接进入事件
	node.GetOrCreatePeer(whoAmI).InConnect(conn)
}

func (node * Node) getService(name string) chan Request {
	node.mutex.Lock()
	defer node.mutex.Unlock()

	if service , ok := node.services[name]; ok {
		return service
	} else {
		return nil
	}
}

func (node *Node) dipatchMessage(source string,msg *Message, response chan Message) {
	service := node.getService(msg.Method)

	if service == nil {
		WARN.Printf("peer(%s) try call service(%s) err : not found\n",source,msg.Method)
		return
	}

	service <- Request{msg.Id,bytes.NewBuffer(msg.Payload),node,response}
}


