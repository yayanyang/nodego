package nodego

import(
	"io"
	"net"
	"time"
	"sync"
	"bytes"
	"encoding/binary"
	. "github.com/yayanyang/nodego/log"
	timer "github.com/yayanyang/nodego/timer"
)


type Peer interface {
	InConnect(conn net.Conn)
	SendRequest(name string,request interface{}) (Future, error)
	SendRequestWithTimeout(name string,timeout time.Duration,request interface{}) (Future, error)
	Post(name string,request interface{}) error
}

type peer struct {
	name        string
	seq         id // id sequence
	node 		*Node
	waits		map[id] chan Response //wait list
	send 		chan Message   //发送队列 
	mutex 		sync.Mutex
}

func (p *peer) recvLoop(conn net.Conn) {
	for {
		msg , err := p.node.ReadMessage(conn);
		if err != nil {
			//读取错误
			ERROR.Printf("close read connection, recv message from peer(%s) err :%s\n",p.name,err)
			conn.Close()
			return
		}

		switch msg.Code{
		case request :
			go p.node.dipatchMessage(p.name, msg,p.send)
		case notify :
			go p.node.dipatchMessage(p.name, msg,nil)
		case response:
			go p.notify(msg.Id,bytes.NewBuffer(msg.Payload),nil)
		}
	}
}

func (p * peer) sendLoop(conn net.Conn){
	for {
		msg := <- p.send

		TRACE.Printf("send message on write connection(%s)",conn.RemoteAddr())

		if err := p.sendMessage(conn,msg); err != nil {

			ERROR.Printf("close write connection(%s) ,write data err:%s",conn,err)
			conn.Close()
			go p.OutConnect()
			break
		}

		TRACE.Printf("send message on write connection(%s) -- success",conn.RemoteAddr())
	}
}

func (p *peer) InConnect(conn net.Conn) {
	go p.recvLoop(conn)
}

func (p *peer) sendMessage(conn net.Conn, msg Message) error{
	var buff bytes.Buffer
	p.node.encoding.Encode(&buff,msg)
	header := make([]byte,2)
	binary.BigEndian.PutUint16(header,uint16(buff.Len()))

	//write header
	if _, err := conn.Write(header); err != nil {
		return err
	}
	//write body

	if _, err := io.Copy(conn,&buff); err != nil {
		return err
	}

	return nil

}

func (p * peer) sendWhoAmI(conn net.Conn) bool {

	p.mutex.Lock()

	id := p.seq

	p.seq ++

	p.mutex.Unlock()

	var buff bytes.Buffer

	listenAddr,_ := net.ResolveTCPAddr("tcp", p.node.name)

	connAddr ,_ := net.ResolveTCPAddr("tcp", conn.LocalAddr().String())

	name := net.TCPAddr{connAddr.IP,listenAddr.Port,connAddr.Zone}

	p.node.encoding.Encode(&buff,name.String());

	msg := Message{Code:whoAmI,Id:id,Payload:buff.Bytes()}

	
	INFO.Printf("send whoAmI on write connection(%s)",conn.RemoteAddr())

	if err := p.sendMessage(conn,msg); err != nil {
		ERROR.Printf("close write connection(%s) ,write whoAmI err:%s",conn,err)
		conn.Close()
		return false
	}

	INFO.Printf("send whoAmI on write connection(%s) -- success",conn.RemoteAddr())

	return true
}

func (p * peer) OutConnect() {
	for {
		INFO.Printf("try establish write connection(%s)",p.name)

		conn, err := net.Dial("tcp", p.name)

		if err != nil {
			WARN.Printf("try establish write connection(%s) err:%s",p.name,err)
			time.Sleep(connRetryTimeout)
			continue
		}

		INFO.Printf("established write connection(%s)",conn.RemoteAddr())

		if p.sendWhoAmI(conn){
			p.sendLoop(conn)
		}
		
	}
}

func (p * peer) createFuture() (chan Response,id) {
	p.mutex.Lock()

	defer p.mutex.Unlock()

	id := p.seq

	p.seq ++

	future := make(chan Response,1)

	p.waits[id] = future

	return future,id
}

func (p *peer) notify(target id, reader io.Reader, err error) {
	p.mutex.Lock()

	defer p.mutex.Unlock()

	if future , ok := p.waits[target]; ok {
		future <- Response{Error : err,reader:reader,node:p.node}
	}
}

func (p *peer) sendRequest(msg Message) error {
	//TODO: 可能会阻塞，是否需创建协程来处理该操作？
	p.send <- msg

	return nil
}

func (p *peer) SendRequest(name string,payload interface{}) (Future, error) {
	future,id := p.createFuture()

	var buff bytes.Buffer

	p.node.encoding.Encode(&buff,payload);

	return future, p.sendRequest(Message{request,id,name,buff.Bytes()})
}


func (p *peer) Post(name string,payload interface{}) error {

	p.mutex.Lock()

	id := p.seq

	p.seq ++

	p.mutex.Unlock()

	var buff bytes.Buffer

	p.node.encoding.Encode(&buff,payload);

	return p.sendRequest(Message{notify,id,name,buff.Bytes()})
}

func (p * peer) SendRequestWithTimeout(name string,timeout time.Duration,payload interface{}) (Future, error){
	future,id := p.createFuture()

	var buff bytes.Buffer

	p.node.encoding.Encode(&buff,payload);

	timer.Timeout(timeout, func () {
		p.notify(id,nil,RequestTimeout)
	})

	return future,p.sendRequest(Message{request,id,name,buff.Bytes()})
}


//create new peer object
func createPeer(node *Node,name string) Peer{

	p  := &peer{
		name:name,
		node:node, 
		send : make(chan Message,sendCacheSize),
		waits : make(map[id] chan Response) }

	go p.OutConnect()

	return p
}