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

package ants_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
)

const (
	RunTimes      = 1000000
	benchParam    = 10
	benchAntsSize = 20000
)

//现在看一下bench的写法. 首先是自定义测试函数.

// 第一个是每一个睡10微秒
func demoFunc() {
	n := 10
	time.Sleep(time.Duration(n) * time.Millisecond)
}

// 第二个是睡args微秒.
func demoPoolFunc(args interface{}) {
	n := args.(int)
	time.Sleep(time.Duration(n) * time.Millisecond)
}

// 不停的让出时间片.
func longRunningFunc() {
	for {
		runtime.Gosched()
	}
}

func longRunningPoolFunc(arg interface{}) {
	if ch, ok := arg.(chan struct{}); ok { //arg强制转化为channel
		<-ch //然后ch取一个数据.
		return
	}
	for {
		runtime.Gosched() //然后让出时间片.
	}
}

// 下面是bench函数.
func BenchmarkGoroutineWithFunc(b *testing.B) {
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			go func() {
				demoPoolFunc(benchParam)
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkSemaphoreWithFunc(b *testing.B) {

	// 关于b.N这个东西，为了让程序达到稳态，在benchmark跑的过程中N是会一直变化的，所以一定要让程序稳态进行，如果出现了非稳态的状况，它就会一直跑不完。所以要保证你的benchmark在一定时间内达到稳态，如果这段不明白，看下面的例子。
	var wg sync.WaitGroup
	sema := make(chan struct{}, benchAntsSize) //benchAntsSize表示一次处理多少个.

	for i := 0; i < b.N; i++ { //b.N是测试数量,他是程序自己设置的.不用管他.
		//里面写每一次测试运行的逻辑
		wg.Add(RunTimes) //一共运行RunTimes次,
		for j := 0; j < RunTimes; j++ {
			sema <- struct{}{}
			go func() {
				demoPoolFunc(benchParam)
				<-sema
				wg.Done() //每次运行完计数器-1
			}()
		}
		wg.Wait()
	}
}

// 下面是使用ant的版本.~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
func BenchmarkAntsPoolWithFunc(b *testing.B) {
	var wg sync.WaitGroup //之前的chanel用大小为benchAntsSize的pool来设置
	p, _ := ants.NewPoolWithFunc(benchAntsSize, func(i interface{}) {
		demoPoolFunc(i)
		wg.Done()
	})//p等于是吧demoPoolFunc包了一层pool来用.
	defer p.Release()

	b.StartTimer()
	for i := 0; i < b.N; i++ {

		//这块是每一次测试的代码.
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			_ = p.Invoke(benchParam) /// Invoke 提交一个任务给p, 是传进去的参数.
		}
		wg.Wait()

	}
	b.StopTimer()
}

func BenchmarkGoroutineThroughput(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 0; j < RunTimes; j++ {
			go demoPoolFunc(benchParam)
		}
	}
}

func BenchmarkSemaphoreThroughput(b *testing.B) {
	sema := make(chan struct{}, benchAntsSize)
	for i := 0; i < b.N; i++ {
		for j := 0; j < RunTimes; j++ {
			sema <- struct{}{}
			go func() {
				demoPoolFunc(benchParam)
				<-sema
			}()
		}
	}
}

func BenchmarkAntsPoolThroughput(b *testing.B) {
	p, _ := ants.NewPoolWithFunc(benchAntsSize, demoPoolFunc)
	defer p.Release()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < RunTimes; j++ {
			_ = p.Invoke(benchParam)
		}
	}
	b.StopTimer()
}
