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
		time.Sleep(time.Second * 5)
		cond.L.Lock()
		flag = true
		cond.Signal()
		cond.L.Unlock()
	}()

	fmt.Println("waiting")
	cond.L.Lock()
	for !flag {//falg为false时候一直等待,用法是wait上下要用lock , unlock才行.
		cond.Wait()
	}
	cond.L.Unlock()
	fmt.Println("done")
}