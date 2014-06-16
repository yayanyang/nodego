package timer

import ( 
	"time"
	"sync"
	"fmt"
	"unsafe"
)

const(
	INVALID_TICKS = 1 << unsafe.Sizeof(int(0)) * 8 - 1
)


type Timer struct{
	next 		*Timer
	prev		**Timer
	ticks 		uint
	callback 	func()
}

type cascade struct{
	cursor 		uint
	buckets		[256] *Timer
}

type timewheel struct{
	timers		int
	cascades	[4]cascade
}

const nsOfTick = time.Millisecond * 10 //时间段最小单位10ms

var __globalTimewheel timewheel

var __globalTimewheelOnce sync.Once

var __globalTimewheelMutex sync.Mutex

func (tw *timewheel)__invoke(timer *Timer) {
	__globalTimewheelMutex.Unlock()

	defer func () {
		if r := recover(); r != nil {
            fmt.Printf("Runtime error caught: %v", r)
        }

        __globalTimewheelMutex.Lock()

        tw.timers -= 1

        timer.ticks = INVALID_TICKS
	}()

	timer.callback()
}

func (tw *timewheel) cascade( index int ) {
	cas := &tw.cascades[index];

	if cas.cursor != 0 || index == 3 {
		return
	}

	index += 1

	upper := &tw.cascades[index]

	timers := upper.buckets[upper.cursor]

	upper.buckets[upper.cursor] = nil

	upper.cursor += 1

	tw.cascade(index)

	

	for timers != nil {
		next := timers.next
		switch(index){
		case 1:
			timers.ticks &= 0xff
		case 2:
			timers.ticks &= 0xffff
		case 3:
			timers.ticks &= 0xffffff
		default:
			panic("inner error")
		}

		timers.next = nil
		timers.prev = nil

		tw.insert(timers)
		timers = next
	}
}

func (tw *timewheel) tick() {

	__globalTimewheelMutex.Lock()

	defer __globalTimewheelMutex.Unlock()

	cas := &tw.cascades[0]
	timers := cas.buckets[cas.cursor]
	cas.buckets[cas.cursor] = nil
	cas.cursor += 1
	cas.cursor %= 256
	tw.cascade(0)

	for timers != nil {

		next := timers.next
		timers.next = nil
		timers.prev = nil
		tw.__invoke(timers)
		timers = next
	}
}

func (tw *timewheel) insert( timer * Timer ) *Timer {

	var buckets uint

	var cas *cascade

	ticks := timer.ticks

	tw.timers += 1

	if ticks == 0 {
		tw.__invoke(timer)
		return timer
	}

	if ((ticks >> 24) & 0xff ) != 0 {
        buckets = ((ticks >> 24) & 0xff);

        cas = &tw.cascades[3]

    } else if ((ticks >> 16) & 0xff) != 0{
        buckets = ((ticks >> 16) & 0xff);

        cas = &tw.cascades[2]
    } else if ((ticks >> 8) & 0xff) != 0 {
        buckets = ((ticks >> 8) & 0xff);

        cas = &tw.cascades[1]
    } else {
        buckets = ticks;

        cas = &tw.cascades[0]
    }

    buckets = (buckets + cas.cursor - 1) % 256

    timer.next = cas.buckets[buckets]

    timer.prev = &cas.buckets[buckets]

    if cas.buckets[buckets] != nil {

        cas.buckets[buckets].prev = &timer.next;
    }

    cas.buckets[buckets] = timer;

    return timer
}

func Timeout(timeout time.Duration, callback func()) *Timer{

	__globalTimewheelOnce.Do(func () {
		__globalTimewheel = timewheel {}

		go func() {
			
			for{

				__globalTimewheel.tick()
				time.Sleep(nsOfTick)
			}

		}()
	})

	__globalTimewheelMutex.Lock()

	defer __globalTimewheelMutex.Unlock()

	return __globalTimewheel.insert(&Timer{ticks: uint(timeout / nsOfTick), callback : callback})
}


func (timer *Timer) Expired() bool {
	return timer.ticks == INVALID_TICKS
}

func (timer *Timer) Cancel() {
	__globalTimewheelMutex.Lock()
	defer __globalTimewheelMutex.Unlock()

	*timer.prev = timer.next

	if timer.next != nil {
		timer.next.prev = timer.prev
	}
}
