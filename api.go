/* Package nodego
 * nodego go语言实现的轻量级mico-service framework
 * nodego 支持两种rpc调用方式：
 * 			Call : Request - Response 模式，同步调用方式通过Node#SetRequestTimeout来控制超时
 * 			Notify : 异步调用，没有响应确认。
 * nodego 节点之间通过两条TCP长连接进行通讯，分别对应读写操作
 */
package nodego

import (
	"bytes"
	"encoding/binary"
	"errors"
	. "github.com/yayanyang/nodego/log"
	"io"
	"net"
	"sync"
	"time"
)

//错误代码
var (
	RequestTimeout   = errors.New("request timeout")
	ServiceNameError = errors.New("serivce name error")
)

//消息ID
type Id uint32

//消息类型
type Code uint8

//节点之间通讯消息结构体
type Message struct {
	Code    Code   //消息类型
	Id      Id     //消息ID，两个节点之间的消息ID应该是单调递增但是并不要求全局唯一
	Method  string //调用方法名
	Payload []byte //消息体
}

//Encoding nodego底层消息传输使用的序列化接口
//用户可以通过定制该接口，替换消息传输使用的序列化算法
type Encoding interface {
	Decode(reader io.Reader, e interface{}) error
	Encode(writer io.Writer, e interface{}) error
}

//Peer 对端代理对象接口
type Peer interface {
	InConnect(conn net.Conn)                                      //通知对端有新的读TCP连接进入
	SendRequest(name string, request interface{}) (Future, error) //通过对端代理对象向对端发送一条Request消息
	Post(name string, request interface{}) error                  //通过对端代理对象向对端发送一条Notify消息
	SetRequestTimeout(timeout time.Duration) time.Duration        //设置Request超时
}

//Node nodgo节点对象
type Node struct {
	name     string                  //节点监听绑定地址
	services map[string]chan Request //注册服务的请求通道集合
	peers    map[string]Peer         //对端代理对象集合
	listener net.Listener            //节点监听tcp socket
	encoding Encoding                //序列化接口对象
	mutex    sync.Mutex              //节点同步对象
}

//异步Future模式的实现对象，它是一个通道对象
type Future <-chan Response

//Response 应答封装对象
//当调用Node#Call方法时，可以通过等待返回的Future对象来获得它
type Response struct {
	Error  error //错误代码，一般来说是RequestTimeout
	reader io.Reader
	node   *Node
}

//Get 获得反序列化后的应答对象，该方法不可重入
func (response *Response) Get(e interface{}) error {
	return response.node.encoding.Decode(response.reader, e)
}

//Request 请求对象
//框架回调注册的服务函数时，将该对象作为参数传入
type Request struct {
	id       Id
	reader   io.Reader
	node     *Node
	response chan Message
}

//Get 获取反序列化后的请求对象，该方法不可重入
func (request *Request) Get(e interface{}) error {
	return request.node.encoding.Decode(request.reader, e)
}

//New 创建一个新的nodgo节点对象，并将监听端口绑定到laddr上面
func New(laddr string) (*Node, error) {
	//创建监听socket
	var (
		listener net.Listener
		err      error
	)
	if listener, err = net.Listen("tcp", laddr); err != nil {
		ERROR.Printf("create tcp(%s) listen socket error :%s\n", laddr, err)
		return nil, err
	}

	return &Node{
		name:     laddr,
		services: make(map[string]chan Request),
		peers:    make(map[string]Peer),
		listener: listener,
		encoding: &gobEncoding{}}, nil
}

//Run 启动nodego对象，同步方法节点停止运行前该方法不会返回
func (node *Node) Run() {
	for {
		conn, err := node.listener.Accept()

		if err != nil {
			ERROR.Printf("call tcp(%s) accept error :%s\n", node.listener, err)
			continue
		}
		TRACE.Printf("accept read connection(%s)", conn.RemoteAddr())
		go node.handshake(conn)
	}
}

//Register 向nodego对象注册服务函数
//同一个nodego对象中服务名不能重复，否则注册失败并返回ServiceNameError错误
func (node *Node) Register(name string, call func(request Request) interface{}) error {

	node.mutex.Lock()

	defer node.mutex.Unlock()

	if _, ok := node.services[name]; ok {
		return ServiceNameError
	}

	serviceChan := make(chan Request, sendCacheSize)

	go func() {
		for {
			if !serviceCall(name, serviceChan, call) {
				break
			}
		}
	}()

	node.services[name] = serviceChan

	return nil
}

//Call rpc调用
//同步调用远程节点(target)注册的服务(method)
//通过 response := <- future 的方式获取应答对象，该调用将阻塞当前gocoroutine。
func (node *Node) Call(target string, method string, data interface{}) (Future, error) {
	peer := node.GetOrCreatePeer(target)

	return peer.SendRequest(method, data)
}

//SetCallTimeout 设置Call方法的超时时间
//超时发生时客户端收到的Response#Error将被设置为RequestTimeout
//当客户端收到RequestTimeout错误时，实际调用成功也是有可能的；所以适当的超时时间设置很重要
func (node *Node) SetCallTimeout(target string, timeout time.Duration) time.Duration {
	peer := node.GetOrCreatePeer(target)

	return peer.SetRequestTimeout(timeout)
}

//Notify rpc调用
//异步调用远程节点(target)注册的服务(method)
//该方法忽略远程服务的返回值，同时意味着客户端无法确认该方法是否调用成功
func (node *Node) Notify(target string, method string, data interface{}) error {
	peer := node.GetOrCreatePeer(target)

	return peer.Post(method, data)
}

//ReadMessage 在某个连接上读取一条消息
func (node *Node) ReadMessage(conn net.Conn) (*Message, error) {
	header := make([]byte, 2)
	//读取消息头，两个字节大端序。表示消息体长度
	if _, err := io.ReadFull(conn, header); err != nil {
		ERROR.Printf("read message length from tcp(%s) error :%s\n", conn, err)
		return nil, err
	}

	//转换字节序,最长65535
	//TODO:限制最长报文到一个合理范围
	length := binary.BigEndian.Uint16(header)

	body := make([]byte, length)

	if readlen, err := io.ReadFull(conn, body); err != nil {
		ERROR.Printf("read message body(%d:%d) from tcp(%s) error :%s\n", length, readlen, conn.RemoteAddr(), err)
		return nil, err
	}

	var msg Message

	if err := node.encoding.Decode(bytes.NewBuffer(body), &msg); err != nil {
		ERROR.Printf("decode message body(%d) from tcp(%s) error :%s,%v", length, conn.RemoteAddr(), err, body)
		return nil, err
	}

	return &msg, nil
}

//GetOrCreatePeer 获得或者创建一个新的对端代理对象
func (node *Node) GetOrCreatePeer(name string) Peer {
	node.mutex.Lock()
	defer node.mutex.Unlock()

	if peer, ok := node.peers[name]; ok {
		//查询到已经创建对端代理对象，直接返回该对象
		return peer
	} else {
		//创建新的对端代理对象
		peer = createPeer(node, name)
		node.peers[name] = peer
		return peer
	}
}
