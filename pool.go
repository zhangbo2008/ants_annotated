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
	"sync"
	"sync/atomic"
	"time"
)

// 最本质的函数都是这里面的. 从pool.go开始读!!!!!!!!!!!!!!!!!!!
// Pool accept the tasks from client, it limits the total of goroutines to a given number by recycling goroutines. 他是任务的池子,池子里面限制go线程的数量,从而循环使用.
type Pool struct {
	// capacity of the pool.
	capacity int32 //最大使用量

	// running is the number of the currently running goroutines.
	running int32 //当前使用量, 正在工作的工人数量.

	// expiryDuration set the expired time (second) of every worker.
	expiryDuration time.Duration //其实是,每一次gc的时间间隔.

	// workers is a slice that store the available workers.
	workers []*goWorker //所有闲置工人组成的数组,他们都是没任务的,挂机中.

	// release is used to notice the pool to closed itself.
	release int32

	// lock for synchronous operation.
	lock sync.Mutex

	// cond for waiting to get a idle worker.
	cond *sync.Cond

	// once makes sure releasing this pool will just be done for one time.
	once sync.Once //保证只运行一次.

	// workerCache speeds up the obtainment of the an usable worker in function:retrieveWorker.
	workerCache sync.Pool //加速对象的创建. 那么这个里面拿到的对象是不是workers里面的呢?????????????我感觉应该不是.因为那里面写了,他里面的东西是不用管的.

	// panicHandler is used to handle panics from each worker goroutine.
	// if nil, panics will be thrown out again from worker goroutines.
	panicHandler func(interface{})

	// Max number of goroutine blocking on pool.Submit.
	// 0 (default value) means no such limit.
	maxBlockingTasks int32

	// goroutine already been blocked on pool.Submit
	// protected by pool.lock
	blockingNum int32

	// When nonblocking is true, Pool.Submit will never be blocked.
	// ErrPoolOverload will be returned when Pool.Submit cannot be done at once.
	// When nonblocking is true, MaxBlockingTasks is inoperative.
	nonblocking bool
}

// 周期性的清楚过期的worker
// Clear expired workers periodically.
func (p *Pool) periodicallyPurge() {
	heartbeat := time.NewTicker(p.expiryDuration) //每p.expiryDuration 秒会把当前时间放到heartbeat里面.
	defer heartbeat.Stop()

	var expiredWorkers []*goWorker
	for range heartbeat.C { //这个会一直运行直到heartbeat析构.
		if atomic.LoadInt32(&p.release) == CLOSED { //查看pool是否已经析构了.
			break
		}
		currentTime := time.Now()

		//锁上pool进行析构.
		p.lock.Lock()
		idleWorkers := p.workers
		n := len(idleWorkers)
		var i int
		for i = 0; i < n && currentTime.Sub(idleWorkers[i].recycleTime) > p.expiryDuration; i++ {
		} //这行代码找到那个最大索引i. 注意 channel里面加worker肯定是创建时间递增的.
		expiredWorkers = append(expiredWorkers[:0], idleWorkers[:i]...) //过期的worker也加入expierd里面.      ...表示idleWorkers的元素被打散一个个append进strs
		if i > 0 {                                                      //整理idelworker
			m := copy(idleWorkers, idleWorkers[i:]) //m表示成功拷贝了多少个.
			for i = m; i < n; i++ {
				idleWorkers[i] = nil
			} //为啥还需要补nil,因为nil是情况里面的worker内部元素.保证了内存不泄露.保证里面东西也析构了.所以必须补nil
			p.workers = idleWorkers[:m]
		}
		p.lock.Unlock()

		// Notify obsolete workers to stop.
		// This notification must be outside the p.lock, since w.task
		// may be blocking and may consume a lot of time if many workers
		// are located on non-local CPUs.
		for i, w := range expiredWorkers {
			w.task <- nil //添加nil是作为结尾信号.
			expiredWorkers[i] = nil
		}

		// There might be a situation that all workers have been cleaned up(no any worker is running)
		// while some invokers still get stuck in "p.cond.Wait()",
		// then it ought to wakes all those invokers.
		if p.Running() == 0 { //有些worker已经删除了,但是还有cond在等,所以要激活他们.
			p.cond.Broadcast()
		}

	}
}

// NewPool generates an instance of ants pool.
func NewPool(size int, options ...Option) (*Pool, error) {
	if size <= 0 {
		return nil, ErrInvalidPoolSize
	}

	opts := new(Options)
	//把options设置上.
	for _, option := range options {
		option(opts)
	}

	if expiry := opts.ExpiryDuration; expiry < 0 {
		return nil, ErrInvalidPoolExpiry
	} else if expiry == 0 {
		opts.ExpiryDuration = time.Duration(DEFAULT_CLEAN_INTERVAL_TIME) * time.Second
	}

	var p *Pool
	if opts.PreAlloc {
		p = &Pool{
			capacity:         int32(size),
			expiryDuration:   opts.ExpiryDuration,
			workers:          make([]*goWorker, 0, size),
			nonblocking:      opts.Nonblocking,
			maxBlockingTasks: int32(opts.MaxBlockingTasks),
			panicHandler:     opts.PanicHandler,
		}
	} else {
		p = &Pool{
			capacity:         int32(size),
			expiryDuration:   opts.ExpiryDuration,
			nonblocking:      opts.Nonblocking,
			maxBlockingTasks: int32(opts.MaxBlockingTasks),
			panicHandler:     opts.PanicHandler,
		}
	}
	p.cond = sync.NewCond(&p.lock)

	// Start a goroutine to clean up expired workers periodically.
	go p.periodicallyPurge()

	return p, nil
}

//---------------------------------------------------------------------------

// Submit submits a task to this pool.
func (p *Pool) Submit(task func()) error {
	if atomic.LoadInt32(&p.release) == CLOSED {
		return ErrPoolClosed
	}
	if w := p.retrieveWorker(); w == nil {
		return ErrPoolOverload
	} else {
		w.task <- task
	}
	return nil
}

// Running returns the number of the currently running goroutines.
func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

// Free returns the available goroutines to work. 返回还能工作的线程多少个.
func (p *Pool) Free() int {
	return int(atomic.LoadInt32(&p.capacity) - atomic.LoadInt32(&p.running))
}

// Cap returns the capacity of this pool.
func (p *Pool) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

// Tune changes the capacity of this pool.
func (p *Pool) Tune(size int) {
	if p.Cap() == size {
		return
	}
	atomic.StoreInt32(&p.capacity, int32(size))
}

// Release Closes this pool.
func (p *Pool) Release() {
	p.once.Do(func() {
		atomic.StoreInt32(&p.release, 1)
		p.lock.Lock()
		idleWorkers := p.workers
		for i, w := range idleWorkers { //析构所有worker
			w.task <- nil
			idleWorkers[i] = nil
		}
		p.workers = nil
		p.lock.Unlock()
	})
}

//---------------------------------------------------------------------------

// incRunning increases the number of the currently running goroutines.
func (p *Pool) incRunning() {
	atomic.AddInt32(&p.running, 1)
}

// decRunning decreases the number of the currently running goroutines.
func (p *Pool) decRunning() {
	atomic.AddInt32(&p.running, -1) //能用atomic的就用atomic,比锁更快.
}

// 从池子中分配出一个worker.
// retrieveWorker returns a available worker to run the tasks.
func (p *Pool) retrieveWorker() *goWorker {
	var w *goWorker
	spawnWorker := func() {
		//如果cache里面能get到东西,那么就使用get的,然后指针强转即可.
		if cacheWorker := p.workerCache.Get(); cacheWorker != nil {
			w = cacheWorker.(*goWorker)
		} else {
			w = &goWorker{ //否则用构造函数.
				pool: p,
				task: make(chan func(), workerChanCap),
			}
		}
		w.run() //让worker启动.实际上他就会阻塞等待任务给他,他就会运行.
	}
	//下面开始具体运行
	p.lock.Lock()
	idleWorkers := p.workers
	n := len(idleWorkers) - 1 //工人最大索引
	if n >= 0 {               //第一种情况是有闲置的工人.
		w = idleWorkers[n]
		idleWorkers[n] = nil
		p.workers = idleWorkers[:n] //那么工人池子中分配一个即可.
		p.lock.Unlock()
	} else if p.Running() < p.Cap() { //如果池子没有工人,并且cap足够大可以创建工人,那么我们就穿件工人.
		p.lock.Unlock() //这里面的逻辑我注意到.他是先unlock再spwanworker的. 这是因为spawnWorker函数里面是自己并发安全的.没必要再持有锁了. 并发编程的原则是让lock和unlock之间的代码行数尽可能的小!!!!!!!!这样才能最大的提高性能.
		spawnWorker()
	} else { //这就是ca已经满了.
		if p.nonblocking { //如果p要求非阻塞,那么直接返回创建失败即可
			p.lock.Unlock()
			return nil
		}
	Reentry: //这个逻辑处理阻塞情况.
		//先查看阻塞的数量跟最大阻塞的设置之前的关系.已经到达最大的话,我们就只能返回nil.
		if p.maxBlockingTasks != 0 && p.blockingNum >= p.maxBlockingTasks {
			p.lock.Unlock()
			return nil
		}
		p.blockingNum++
		p.cond.Wait() //让p阻塞.知道其他进程触发signal函数. 触发之后blocknum--
		p.blockingNum--
		if p.Running() == 0 { //还是一样逻辑,没有了就创建.那么这时候为什么不需要查询空闲工人呢?????????????????因为308行触发278之后的代码.这时候.工人回到池子中.为什么不冲池子works中拿,而是直接创建呢????????????????????很奇怪!!!!!!!!!!!!!!!
			p.lock.Unlock()
			spawnWorker()
			return w
		} //下面从池子中拿.
		l := len(p.workers) - 1 //到这里说明running的存在,那么我们再检查一遍空闲工人.
		if l < 0 {              //说明还是没有空闲的.就一直死循环.继续重新等278行的信号.
			goto Reentry
		}
		//运行到这里说明有空闲的了.
		w = p.workers[l]
		p.workers[l] = nil
		p.workers = p.workers[:l]
		p.lock.Unlock()
	}
	return w
}

// revertWorker puts a worker back into free pool, recycling the goroutines.
func (p *Pool) revertWorker(worker *goWorker) bool {
	if atomic.LoadInt32(&p.release) == CLOSED || p.Running() > p.Cap() {
		return false
	}
	worker.recycleTime = time.Now()
	p.lock.Lock()//注意cond这个锁是跟他上下lock, unlock一起组成一个锁!!!!!!!!跟本质的原因可以看源码!!!!!!!!!!!!!所以从这个用法知道304和278不会发生死锁.这两个函数本质都是被一个锁锁上的.
	p.workers = append(p.workers, worker) //放回池子,触发信号.

	// Notify the invoker stuck in 'retrieveWorker()' of there is an available worker in the worker queue.
	p.cond.Signal() // cond: wait是等待,  signal是唤醒, Broadcast是全唤醒.
	p.lock.Unlock()
	return true
}
