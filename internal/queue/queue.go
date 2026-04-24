package queue

import (
	"context"
	"errors"
	"log"
	"sync"

	"github.com/example/pr-ai-teammate/internal/orchestrator"
)

var ErrFull = errors.New("analysis queue is full")

const DefaultBufferSize = 100

// Worker is satisfied by *orchestrator.Service.
type Worker interface {
	AnalyzePR(ctx context.Context, input orchestrator.AnalyzeInput) (orchestrator.AnalyzeResult, error)
}

type Queue struct {
	jobs chan orchestrator.AnalyzeInput
	wg   sync.WaitGroup
}

func New(bufferSize int) *Queue {
	return &Queue{jobs: make(chan orchestrator.AnalyzeInput, bufferSize)}
}

// Enqueue adds a job to the queue. Returns ErrFull if the buffer is exhausted.
func (q *Queue) Enqueue(input orchestrator.AnalyzeInput) error {
	select {
	case q.jobs <- input:
		return nil
	default:
		return ErrFull
	}
}

// Start launches the background worker goroutine. ctx controls cancellation of
// in-progress AnalyzePR calls; the worker exits after draining when Shutdown is called.
func (q *Queue) Start(ctx context.Context, worker Worker) {
	q.wg.Add(1)
	go q.run(ctx, worker)
}

// Shutdown closes the job channel and blocks until the worker has drained all
// queued jobs. Call this after the HTTP server has stopped accepting requests.
func (q *Queue) Shutdown() {
	close(q.jobs)
	q.wg.Wait()
}

func (q *Queue) run(ctx context.Context, worker Worker) {
	defer q.wg.Done()
	for input := range q.jobs {
		result, err := worker.AnalyzePR(ctx, input)
		if err != nil {
			log.Printf("analysis failed for %s#%d: %v", input.Repository, input.PullNumber, err)
			continue
		}
		log.Printf("analysis complete: %s", result.Summary)
	}
}
