package workerpool

import (
	"context"
	"log"
	"os"
	"sync"

	wp "github.com/dc0d/workerpool"
)

var pool = New()

// Send enqueues the given job and returns its ID.
func Send(job *Job) string {
	return pool.Send(job)
}

// GetJob returns the job for the given id.
func GetJob(id string) *Job {
	return pool.GetJob(id)
}

// GetJobStatus returns the job's status for the given id.
func GetJobStatus(id string) string {
	return pool.GetJobStatus(id)
}

// CancelJob stops the job for the given id in an asynchronous routine.
func CancelJob(id string) {
	pool.CancelJob(id)
}

// GetPoolSize returns the number of running workers.
func GetPoolSize() int {
	return pool.GetPoolSize()
}

// SetPoolSize defines the number of wanted workers.
// n is absolute so the pool can be expanded or shrunk according to n.
func SetPoolSize(n int) {
	pool.SetPoolSize(n)
}

// GetJobsMetrics returns the metrics about the workerpool.
func GetJobsMetrics() map[string]interface{} {
	return pool.GetJobsMetrics()
}

// SetLogger defines the workerpool logger.
func SetLogger(l Logger) {
	pool.SetLogger(l)
}

// A Workerpool manages asynchronous jobs.
type Workerpool struct {
	log       Logger
	reg       *registry
	jobs      chan func()
	pool      *wp.WorkerPool
	poolMutex sync.Mutex
	workers   []chan struct{}
}

// New instanciates a new Workerpool.
func New() *Workerpool {
	jobs := make(chan func(), 10000)
	p, _ := wp.WithContext(context.Background(), 1, jobs)
	w := &Workerpool{
		pool:    p,
		jobs:    jobs,
		workers: make([]chan struct{}, 0),
		log:     log.New(os.Stdout, "workerpool: ", log.LUTC),
	}
	w.reg = newRegistry(w)

	return w
}

// Send enqueues the given job and returns its ID.
func (w *Workerpool) Send(job *Job) string {
	job.Init(w.log)
	w.reg.add(job)
	job.setStatus(PENDING)
	w.jobs <- job.Run

	return job.ID()
}

// GetJob returns the job for the given id.
func (w *Workerpool) GetJob(id string) *Job {
	return w.reg.get(id)
}

// GetJobStatus returns the job's status for the given id.
func (w *Workerpool) GetJobStatus(id string) string {
	return w.reg.get(id).Status()
}

// CancelJob stops the job for the given id in an asynchronous routine.
func (w *Workerpool) CancelJob(id string) {
	w.reg.cancel(id)
}

// GetPoolSize returns the number of running workers.
func (w *Workerpool) GetPoolSize() int {
	w.poolMutex.Lock()
	defer w.poolMutex.Unlock()

	return len(w.workers) + 1 // initial worker
}

// SetPoolSize defines the number of wanted workers.
// n is absolute so the pool can be expanded or shrunk according to n.
func (w *Workerpool) SetPoolSize(n int) {
	w.poolMutex.Lock()
	defer w.poolMutex.Unlock()

	nbOfWorkers := len(w.workers) + 1 // default worker added

	if n > 0 && n < nbOfWorkers {
		// Shrink
		for _, quitCh := range w.workers[n-1:] {
			go func(quitCh chan struct{}) {
				quitCh <- struct{}{} // Wait current job ending before kill worker
				close(quitCh)        // Drain channel
				w.log.Println("A worker has been stopped")
			}(quitCh)
		}
		w.workers = w.workers[:n-1]
	} else if n > nbOfWorkers {
		// Expand
		n -= nbOfWorkers
		for i := 0; i < n; i++ {
			quit := make(chan struct{}, 0)
			w.pool.Expand(1, 0, quit)
			w.workers = append(w.workers, quit)
			w.log.Println("A worker has been started")
		}
	}
}

// GetJobsMetrics returns the metrics about the workerpool.
func (w *Workerpool) GetJobsMetrics() map[string]interface{} {
	return w.reg.statuses()
}

// SetLogger defines the workerpool logger.
func (w *Workerpool) SetLogger(l Logger) {
	w.log = l
}
