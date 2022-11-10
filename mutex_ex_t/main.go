package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	a := &sync.Mutex{}
	var flag bool
	flag=false
	print(flag)
	go func() {
		time.Sleep(time.Second * 5)
		a.Lock()
		flag = true
		
		a.Unlock()
	}()

	fmt.Println("waiting")
	
	a.Lock()
	time.Sleep(time.Second * 15)
	print("running")
	flag=false
	a.Unlock()
	fmt.Println("done")
}