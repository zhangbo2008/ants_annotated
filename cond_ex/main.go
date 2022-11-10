package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	cond := sync.NewCond(&sync.Mutex{})
	var flag bool
	go func() {
		time.Sleep(time.Second * 1)
		// cond.L.Lock()
		flag = true
		print("before_signal")
		cond.Signal()
		print("after_signal")
		// cond.L.Unlock()
	}()
	time.Sleep(time.Second * 10)
	fmt.Println("waiting")
	cond.L.Lock()
	for !flag { //falg为false时候一直等待,用法是wait上下要用lock , unlock才行.
		cond.Wait()
	}
	cond.L.Unlock()
	fmt.Println("done")
}

//下面我们通过cond源码来解析,为什么cond这么写不会发生死锁.

// func (c *Cond) Wait() {//因为内
// 	c.checker.check()//检测他是否被复制了.
// 	t := runtime_notifyListAdd(&c.notify)//监听
// 	c.L.Unlock()
// 	runtime_notifyListWait(&c.notify, t)
// 	c.L.Lock()
// }
//首先我们看wait函数.  把wati函数替换掉上面主线程的部分.

//主线程变成了------------函数1
// cond.L.Lock()
// for !flag {//falg为false时候一直等待,用法是wait上下要用lock , unlock才行.
// 	c.checker.check()//检测他是否被复制了.
// 	t := runtime_notifyListAdd(&c.notify)//监听
// 	c.L.Unlock()
// 	runtime_notifyListWait(&c.notify, t)
// 	c.L.Lock()
// }
// cond.L.Unlock()

//同事signal函数也替换成源码  判断是否存在等待需要被唤醒的goroutine 没有直接返回

// go func() {-----------------函数2
// 	time.Sleep(time.Second * 5)
// 	cond.L.Lock()
// 	flag = true
// 	c.checker.check()
// 	runtime_notifyListNotifyOne(&c.notify)
// 	cond.L.Unlock()
// }()

//下面我们来模拟1,2的运行.
//假设1先跑.那么会停在56行,锁上c 然后2再跑.?我理解是直接58行能跑.
//假设2跑.
//好像逻辑是signal不会吃锁, 锁上了signal也一样跑.

//// It is allowed but not required for the caller to hold c.L
// during the call.   signal源码写的!!!!!!!!!!!



//看这个https://github.com/golang/go/blob/779c45a50700bda0f6ec98429720802e6c1624e8/src/pkg/runtime/sema.goc



