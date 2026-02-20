package worker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/saravanan611/base/log"
)

type WorkerScall[pJob, pExpected any] struct {
	minWorker, maxWorker, clinetSize, scallPoint, currentEmp int
	cancelFuncs                                              []context.CancelFunc
	job                                                      chan pJob
	do                                                       func(pJob) pExpected
	progress                                                 chan pExpected
	progressFlag                                             bool
	*sync.WaitGroup
	time.Duration
	*time.Ticker
}

func CreateScall[pJob, pExpected any](pScallCycle time.Duration, pMin, pMax, pQSize, pScallPoint int, pFunc func(pJob) pExpected, pExpectedFlab bool) (lWorkerRec *WorkerScall[pJob, pExpected], lErr error) {
	log.Info("CreateScall (+)")
	if pMin < 1 {
		return nil, log.Error("min requed 1 worker")
	}

	if pMax <= pMin {
		return nil, log.Error("max is greater then min val")
	}

	if pQSize < 100 {
		return nil, log.Error("q-size must be greater then 10")
	}

	if pScallPoint < 10 || pScallPoint > pQSize/2 {
		return nil, log.Error("scall-up point is greater then 10 and less then q-size/2")
	}

	if pScallCycle < 5*time.Second {
		return nil, log.Error("scall up time must be greater then or eq to 5 sec")
	}

	lWorkerRec = &WorkerScall[pJob, pExpected]{
		minWorker:  pMin,
		maxWorker:  pMax,
		clinetSize: pQSize,
		scallPoint: pScallPoint,
		do:         pFunc,
		job:        make(chan pJob, pQSize),
		WaitGroup:  &sync.WaitGroup{},
		Duration:   pScallCycle,
	}
	if pExpectedFlab {
		lWorkerRec.progress = make(chan pExpected, pQSize*2)
		lWorkerRec.progressFlag = true
	}
	go lWorkerRec.start()
	log.Info("CreateScall (-)")
	return
}

func (pWorkerRec *WorkerScall[pJob, pExpected]) Do(pWork pJob) {
	// log.Info("Do (+)")
	pWorkerRec.job <- pWork
	pWorkerRec.Add(1)
	// log.Info("Do (-)")
}

func (pWorkerRec *WorkerScall[pJob, pExpected]) IsSpaceIn() bool {
	// log.Info("IsSpaceIn (+)")
	// log.Info("IsSpaceIn (-)")
	return (len(pWorkerRec.job) + pWorkerRec.currentEmp) < pWorkerRec.clinetSize
}

func (pWorkerRec *WorkerScall[pJob, pExpected]) start() {
	pWorkerRec.Ticker = time.NewTicker(pWorkerRec.Duration)
	log.Info("start (+)")
	for range pWorkerRec.Ticker.C {

		pWorkerRec.scallup()
		pWorkerRec.scallDown()
	}
	log.Info("start (-)")
}

func (pWorkerRec *WorkerScall[pJob, pExpected]) Stop() {
	log.Info("Stop (+)")
	pWorkerRec.Wait()
	if pWorkerRec.Ticker != nil {
		pWorkerRec.Ticker.Stop()
	}
	for _, lClose := range pWorkerRec.cancelFuncs {
		lClose()
	}
	close(pWorkerRec.job)
	if pWorkerRec.progressFlag {
		close(pWorkerRec.progress)
	}

	log.Info("Stop (-)")

}

func (pWorkerRec *WorkerScall[pJob, pExpected]) scallup() {
	lCur := int(len(pWorkerRec.job) / pWorkerRec.scallPoint)
	if lCur < pWorkerRec.maxWorker && lCur > pWorkerRec.currentEmp && pWorkerRec.currentEmp < pWorkerRec.maxWorker {
		log.Info("scallup (+)")
		lCtx, lClear := context.WithCancel(context.Background())

		go pWorkerRec.worker(lCtx, pWorkerRec.currentEmp+1)
		pWorkerRec.cancelFuncs = append(pWorkerRec.cancelFuncs, lClear)
		pWorkerRec.currentEmp++
		log.Info("scallup (-)")
	}

}

func (pWorkerRec *WorkerScall[pJob, pExpected]) scallDown() {
	if int(len(pWorkerRec.job)/pWorkerRec.scallPoint) < pWorkerRec.currentEmp && pWorkerRec.currentEmp > pWorkerRec.minWorker {
		log.Info("scallDown (+)")
		lLeastWorker := pWorkerRec.cancelFuncs[len(pWorkerRec.cancelFuncs)-1]
		lLeastWorker()
		pWorkerRec.cancelFuncs = pWorkerRec.cancelFuncs[:len(pWorkerRec.cancelFuncs)-1]
		pWorkerRec.currentEmp--
		log.Info("scallDown (-)")
	}

}

func (pWorkerRec *WorkerScall[pJob, pExpected]) worker(pCtx context.Context, pEmpID int) {
	log.Info("worker %d (+)", pEmpID)
	for {
		select {
		case <-pCtx.Done():
			log.Info("worker %d (-)", pEmpID)
			return
		case lJob := <-pWorkerRec.job:
			log.SetRequestID(uuid.NewString())
			lResp := pWorkerRec.do(lJob)
			if pWorkerRec.progressFlag {
				pWorkerRec.progress <- lResp
			}
			pWorkerRec.Done()
		}
	}

}
