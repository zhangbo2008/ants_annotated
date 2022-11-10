package main

import (
	"fmt"
	"runtime"
)

// Gosched yields the processor, allowing other goroutines to run. It does not suspend the current goroutine, so execution resumes automatically.
// 翻译过来就是他会让出一次时间片,然后cpu空闲时候自动还会自动继续运行这个函数~~~~~~~~~~
func showNumber(i int) {
	runtime.Gosched()
	fmt.Println(i)
}

func main() {

	for i := 0; i < 1000; i++ {
		go showNumber(i)
	}

	runtime.Gosched()
	fmt.Println("Haha")
}
