package server

import (
	"sync"
)

// WorkerPool manages a pool of worker goroutines consuming jobs from a bounded queue.
type WorkerPool struct {
	jobs    chan func()
	workers int
	wg      sync.WaitGroup
	once    sync.Once
}

// NewWorkerPool initializes and starts N workers.
func NewWorkerPool(workers int, queueSize int) *WorkerPool {
	if workers <= 0 {
		workers = 4
	}
	if queueSize <= 0 {
		queueSize = 100
	}

	p := &WorkerPool{
		jobs:    make(chan func(), queueSize),
		workers: workers,
	}

	for i := 0; i < workers; i++ {
		p.wg.Add(1)
		go p.workerLoop()
	}
	return p
}

func (p *WorkerPool) workerLoop() {
	defer p.wg.Done()
	for job := range p.jobs {
		job()
	}
}

// Enqueue adds a job to the pool. Returns true if queued, false if queue is full.
func (p *WorkerPool) Enqueue(job func()) bool {
	select {
	case p.jobs <- job:
		return true
	default:
		return false
	}
}

// Shutdown stops workers and waits for remaining queued jobs to complete.
func (p *WorkerPool) Shutdown() {
	p.once.Do(func() {
		close(p.jobs)
		p.wg.Wait()
	})
}
