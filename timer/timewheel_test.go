package timer

import (
	"testing"
	"crypto/rand"
	"time"
	"sync"
	"fmt"
)

var wg sync.WaitGroup

func print() {
	fmt.Printf("timeout ...")
}

func gen() []byte {
	buff := make([]byte,128)
	rand.Read(buff)
	return buff
}

func notify() {
	fmt.Println("timeout ...")
	wg.Done()
}

var __rand = gen()

func TestAdd(t *testing.T) {

	wg.Add(len(__rand))

	for _,r:= range __rand {
		Timeout(time.Second * time.Duration(r%10),notify)
	}

	wg.Wait()
}

func TestCancel(t *testing.T) {

	wg.Add(1)

	timer := Timeout(time.Second ,notify)

	timer.Cancel()

	Timeout(time.Second ,notify)

	wg.Wait()
}


func BenchmarkAdd(t *testing.B) {

	for i:=1 ; i < len(__rand); i ++ {
		Timeout(time.Second * time.Duration(__rand[i]),print)
	}
}