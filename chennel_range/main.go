package main

// 这个东西对于channel遍历的理解.
import (
	"fmt"
	"time"
)

var ch8 = make(chan int, 6)

func mm1() {
	for i := 0; i < 10; i++ {
		ch8 <- 8 * i
	}
	// close(ch8)
}

func mm2() {
	time.Sleep(3 * time.Second)
	for i := 0; i < 10; i++ {
		ch8 <- 888 * i
	}
	// close(ch8)
}
func main() {

	go mm1()
	go mm2()
	for {
		for data := range ch8 {
			fmt.Print(data, "\t")
		}
		break
	}
}
