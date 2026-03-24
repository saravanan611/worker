package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/saravanan611/log"
)

// WorkerScall is a dynamic auto-scaling worker pool.
// It scales up and down based on queue size relative to scallPoint.
//
// Thread safety: all mutable fields are protected by mu.
type WorkerScall[pJob, pExpected any] struct {
	// immutable after creation — safe to read without lock
	minWorker  int
	maxWorker  int
	clientSize int
	scallPoint int
	do         func(pJob) pExpected

	// channels — created once, safe to read/write concurrently
	job      chan pJob
	Progress chan pExpected
	stopCh   chan struct{}

	progressFlag bool

	// mutable state — must hold mu to read or write
	mu          sync.Mutex
	cancelFuncs []context.CancelFunc
	currentEmp  int

	// WaitGroup tracks in-flight jobs (Add before send, Done after process)
	wg sync.WaitGroup

	// ticker owned by start(); stopped via stopCh
	ticker   *time.Ticker
	duration time.Duration
}

// CreateScall initialises a new auto-scaling worker pool and immediately
// starts minWorker workers so jobs can be processed without waiting for
// the first scale tick.
//
// Parameters:
//
//	pScallCycle   – How often the scaling check runs (minimum 5 s)
//	pMin          – Minimum number of workers (≥ 1)
//	pMax          – Maximum number of workers (> pMin)
//	pQSize        – Job queue buffer size (≥ 100)
//	pScallPoint   – Jobs-per-worker threshold that triggers scale-up/down (10 ≤ x ≤ pQSize/2)
//	pFunc         – Job processing function
//	pProgressFlag – When true, results are forwarded to the Progress channel
func CreateScall[pJob, pExpected any](pScallCycle time.Duration, pMin, pMax, pQSize, pScallPoint int, pFunc func(pJob) pExpected, pProgressFlag bool) (*WorkerScall[pJob, pExpected], error) {
	log.Info("CreateScall (+)")

	if pMin < 1 {
		return nil, fmt.Errorf("min must be at least 1 worker")
	}
	if pMax <= pMin {
		return nil, fmt.Errorf("max (%d) must be greater than min (%d)", pMax, pMin)
	}
	if pQSize < 100 {
		return nil, fmt.Errorf("queue size must be at least 100 (got %d)", pQSize)
	}
	if pScallPoint < 10 || pScallPoint > pQSize/2 {
		return nil, fmt.Errorf("scallPoint must be between 10 and qSize/2 (%d), got %d", pQSize/2, pScallPoint)
	}
	if pScallCycle < 5*time.Second {
		return nil, fmt.Errorf("scallCycle must be >= 5s (got %s)", pScallCycle)
	}

	w := &WorkerScall[pJob, pExpected]{
		minWorker:  pMin,
		maxWorker:  pMax,
		clientSize: pQSize,
		scallPoint: pScallPoint,
		do:         pFunc,
		job:        make(chan pJob, pQSize),
		stopCh:     make(chan struct{}),
		duration:   pScallCycle,
	}

	if pProgressFlag {
		w.Progress = make(chan pExpected, pQSize*2)
		w.progressFlag = true
	}

	// Spawn the minimum number of workers immediately so the pool is
	// ready to process jobs before the first scale tick fires.
	for range pMin {
		w.spawnWorker()
	}

	go w.start()

	log.Info("CreateScall (-)")
	return w, nil
}

// Do submits a job to the worker pool.
// Blocks if the internal queue is full.
// wg.Add must happen before the send so Stop/Wait never races with Done.
func (w *WorkerScall[pJob, pExpected]) Do(job pJob) {
	w.wg.Add(1)
	w.job <- job
}

// IsSpaceIn reports whether there is room in the job queue.
func (w *WorkerScall[pJob, pExpected]) IsSpaceIn() bool {
	return len(w.job) < w.clientSize
}

// Wait blocks until every submitted job has been processed.
func (w *WorkerScall[pJob, pExpected]) Wait() {
	w.wg.Wait()
}

// Stop gracefully shuts down the pool:
//  1. Waits for all in-flight jobs to finish.
//  2. Signals the scale loop to exit.
//  3. Cancels all worker goroutines.
//  4. Closes channels.
func (w *WorkerScall[pJob, pExpected]) Stop() {
	log.Info("Stop (+)")

	// 1. Wait for every submitted job to be processed.
	w.wg.Wait()

	// 2. Stop the scaling loop.
	close(w.stopCh)

	// 3. Cancel all worker goroutines.
	w.mu.Lock()
	for _, cancel := range w.cancelFuncs {
		cancel()
	}
	w.cancelFuncs = nil
	w.currentEmp = 0
	w.mu.Unlock()

	// 4. Close channels (no more sends will happen).
	close(w.job)
	if w.progressFlag {
		close(w.Progress)
	}

	log.Info("Stop (-)")
}

// --------------------------------------------------------------------
// internal helpers
// --------------------------------------------------------------------

// spawnWorker creates one new worker goroutine and registers its cancel func.
// Caller must NOT hold mu — this function acquires it internally.
func (w *WorkerScall[pJob, pExpected]) spawnWorker() {
	ctx, cancel := context.WithCancel(context.Background())

	w.mu.Lock()
	empID := w.currentEmp + 1
	w.cancelFuncs = append(w.cancelFuncs, cancel)
	w.currentEmp++
	w.mu.Unlock()

	go w.worker(ctx, empID)
}

// cancelLastWorker cancels the most-recently-spawned worker and removes it
// from the tracking slice.
// Caller must NOT hold mu — this function acquires it internally.
func (w *WorkerScall[pJob, pExpected]) cancelLastWorker() {
	w.mu.Lock()
	defer w.mu.Unlock()

	last := w.cancelFuncs[len(w.cancelFuncs)-1]
	last()
	w.cancelFuncs = w.cancelFuncs[:len(w.cancelFuncs)-1]
	w.currentEmp--
}

// start runs the periodic scale loop until Stop signals via stopCh.
func (w *WorkerScall[pJob, pExpected]) start() {
	w.ticker = time.NewTicker(w.duration)
	defer w.ticker.Stop()

	log.Info("start (+)")
	for {
		select {
		case <-w.stopCh:
			log.Info("start (-)")
			return
		case <-w.ticker.C:
			w.scaleUp()
			w.scaleDown()
		}
	}
}

// scaleUp spawns additional workers when the queue depth warrants it.
// It may spawn more than one worker per tick if the backlog is large.
func (w *WorkerScall[pJob, pExpected]) scaleUp() {
	for {
		qLen := len(w.job)

		// desired = how many workers the current queue depth needs
		desired := qLen / w.scallPoint
		if desired < w.minWorker {
			desired = w.minWorker
		}
		if desired > w.maxWorker {
			desired = w.maxWorker
		}

		w.mu.Lock()
		cur := w.currentEmp
		w.mu.Unlock()

		if cur >= desired || cur >= w.maxWorker {
			break
		}

		log.Info("scaleUp: spawning worker (cur=%d desired=%d)", cur, desired)
		w.spawnWorker()
	}
}

// scaleDown cancels surplus workers when the queue no longer justifies them.
func (w *WorkerScall[pJob, pExpected]) scaleDown() {
	for {
		qLen := len(w.job)

		// desired = minimum workers needed right now
		desired := max(qLen/w.scallPoint, w.minWorker)

		w.mu.Lock()
		cur := w.currentEmp
		w.mu.Unlock()

		if cur <= desired || cur <= w.minWorker {
			break
		}

		log.Info("scaleDown: cancelling worker (cur=%d desired=%d)", cur, desired)
		w.cancelLastWorker()
	}
}

// worker processes jobs from the shared channel until its context is cancelled.
func (w *WorkerScall[pJob, pExpected]) worker(ctx context.Context, empID int) {
	log.Info("worker %d (+)", empID)
	for {
		select {
		case <-ctx.Done():
			log.Info("worker %d (-)", empID)
			return
		case job, ok := <-w.job:
			if !ok {
				// Channel closed — pool is shutting down.
				log.Info("worker %d: job channel closed, exiting", empID)
				return
			}
			// log.SetRequestID(uuid.NewString())
			result := w.do(job)
			if w.progressFlag {
				w.Progress <- result
			}
			w.wg.Done()
		}
	}
}
