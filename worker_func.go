// MIT License

// Copyright (c) 2018 Andy Pan

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package ants

import (
	"log"
	"runtime"
	"time"
)

// goWorkerWithFunc is the actual executor who runs the tasks,
// it starts a goroutine that accepts tasks and
// performs function calls.
type goWorkerWithFunc struct {
	// pool who owns this worker.
	pool *PoolWithFunc

	// args is a job should be done.
	args chan interface{}

	// recycleTime will be update when putting a worker back into queue.
	recycleTime time.Time //上次回收的时间,也就是他本次任务创建的时间!!!!!!!!!!!
}

// run starts a goroutine to repeat the process
// that performs the function calls.
func (w *goWorkerWithFunc) run() {
	w.pool.incRunning() //计数器加一.
	go func() {
		//defer的理解:defer本身是在某个函数体内执行，比如在函数A内调用了defer func_name()，只要defer func_name()这行代码被执行到了，那func_name这个函数就会被延迟到函数A return或者panic之前执行。  注意：如果是函数是因为调用了os.Exit()而退出，那defer就不会被执行了。 所以defer作为垃圾回收是非常合适的!  defer常用于成对的操作，比如文件打开后要关闭、锁的申请和释放、sync.WaitGroup跟踪的goroutine的计数器的释放等。为了确保资源被释放，可以结合defer一起使用，避免在代码的各种条件分支里去释放资源，容易遗漏和出错。
		defer func() {
			w.pool.decRunning()           //析构时候计数器先-1
			if p := recover(); p != nil { //Recover 是一个Go语言的内建函数，可以让进入宕机流程中的 goroutine 恢复过来，recover 仅在延迟函数 defer 中有效.通常来说，不应该对进入 panic 宕机的程序做任何处理，但有时，需要我们可以从宕机中恢复，至少我们可以在程序崩溃前，做一些操作.  p就是返回的错误信息. 如果panic了,并且返回了错误信息.
				if w.pool.panicHandler != nil {
					w.pool.panicHandler(p)//回调函数.
				} else {
					log.Printf("worker with func exits from a panic: %v\n", p)
					var buf [4096]byte
					n := runtime.Stack(buf[:], false)
					log.Printf("worker with func exits from panic: %s\n", string(buf[:n]))
				}
			}
			w.pool.workerCache.Put(w)//放回pool里面.
		}()

		for args := range w.args {//遍历所有的args跑完func
			if args == nil {
				return
			}
			w.pool.poolFunc(args)
			if ok := w.pool.revertWorker(w); !ok {//然后把w放回池子.
				return
			}
		}
	}()
}
