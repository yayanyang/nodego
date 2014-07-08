package nodego

import (
	"testing"
	"sync"
	//"runtime"
)

func TestCall(t *testing.T) {
	node1,_ := New(":13512") 

	node2,_ := New(":13513")

	node1.Register("mult", func(request Request) interface{} {
		var number int 
		request.Get(&number)
		return number * 2
	})

	node2.Register("mult", func(request Request) interface{} {
		var number int 
		request.Get(&number)
		return number * 2
	})


	go node1.Run()

	go node2.Run()


	//一定要使用"127.0.0.1:13512"形式调用node1服务。
	//node1像node2发送whoAmI消息值为"127.0.0.1:13512"
	//而不是绑定地址(“:13512”)
	future, _ := node2.Call("127.0.0.1:13512","mult",1)

	response := <- future

	var result int

	response.Get(&result)
	
	print("result :",result,"\n")

	future, _ = node1.Call("127.0.0.1:13513","mult",result)

	response = <- future

	response.Get(&result)
	
	print("result :",result,"\n")
}

func TestNoitfy(t *testing.T) {
	node1,_ := New(":13516") 

	node2,_ := New(":13517")

	var wg sync.WaitGroup

	wg.Add(1)

	node1.Register("mult", func(request Request) interface{} {
		var number int 
		request.Get(&number)
		wg.Done()
		return number * 2
	})


	go node1.Run()

	go node2.Run()


	//一定要使用"127.0.0.1:13512"形式调用node1服务。
	//node1像node2发送whoAmI消息值为"127.0.0.1:13512"
	//而不是绑定地址(“:13512”)
	node2.Notify("127.0.0.1:13516","mult",1)

	wg.Wait()
}

var (
	node1 *Node
	node2 *Node
)

func init() {

	//runtime.GOMAXPROCS(runtime.NumCPU())

	node1 , _= New(":13514") 

	node2 ,_ = New(":13515")

	node1.Register("mult", func(request Request) interface{} {
		var number int 
		request.Get(&number)
		return number * 2
	})


	go node1.Run()

	go node2.Run()
}

func BenchmarkCall(b *testing.B) {

	result := 1

	for i := 0; i < b.N; i++ {

		future, _ := node2.Call("127.0.0.1:13514","mult",1)

		response := <- future

		response.Get(&result)
	}

	print("result :",result,"\n")
}