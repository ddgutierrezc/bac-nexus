package mapepiredirect

import (
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	mapepire "github.com/deady54/mapepire-go"
)

const concurrentReads = 10

// RunPool verifies SDK-native two-job reuse and bounded concurrent reads.
func RunPool(cfg Config, out io.Writer) error {
	if cfg.Host == "" || cfg.Port == "" || cfg.User == "" || cfg.Password == "" || out == nil {
		return errors.New("configuration: unavailable")
	}
	server := daemonServer(cfg)
	pool, err := mapepire.NewPool(mapepire.PoolOptions{Creds: server, MaxSize: 2, StartingSize: 2, MaxWaitTime: 5})
	if err != nil {
		return safeError("pool")
	}
	defer pool.Close()

	started := time.Now()
	warmJobs := make([]string, 0, 4)
	for _, token := range []string{"warm-a", "warm-b", "warm-c", "warm-d"} {
		response, executeErr := pool.ExecuteSQLWithOptions("VALUES ?", queryOptions([]string{token}, 1))
		if executeErr != nil || validateCorrelation(response, token) != nil {
			return safeError("pool_warmup")
		}
		warmJobs = append(warmJobs, response.Job)
	}
	distinct, reused, reuseErr := validateReuse(warmJobs)
	if pool.GetJobCount() != 2 || reuseErr != nil {
		return safeError("pool_reuse")
	}
	fmt.Fprintf(out, "pool: jobs=2 distinct_jobs=%d reused_jobs=%d warm_reads=4 reused=true elapsed_ms=%d\n", distinct, reused, time.Since(started).Milliseconds())

	var wg sync.WaitGroup
	started = time.Now()
	errs := make(chan error, concurrentReads)
	for i := range concurrentReads {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			response, executeErr := pool.ExecuteSQLWithOptions("VALUES ?", queryOptions([]string{token}, 1))
			if executeErr != nil {
				errs <- safeError("concurrent_read")
				return
			}
			errs <- validateCorrelation(response, token)
		}(fmt.Sprintf("read-%02d", i))
	}
	wg.Wait()
	close(errs)
	success := 0
	for workerErr := range errs {
		if workerErr != nil {
			return safeError("concurrent_reads")
		}
		success++
	}
	fmt.Fprintf(out, "concurrency: reads=%d correlated=%d elapsed_ms=%d\n", concurrentReads, success, time.Since(started).Milliseconds())
	return nil
}

func validateReuse(jobs []string) (int, int, error) {
	counts := make(map[string]int, 2)
	for _, job := range jobs {
		if job == "" {
			return 0, 0, errors.New("missing job")
		}
		counts[job]++
	}
	if len(counts) != 2 {
		return 0, 0, errors.New("unexpected job distribution")
	}
	for _, count := range counts {
		if count < 2 {
			return 0, 0, errors.New("job not reused")
		}
	}
	return len(counts), len(counts), nil
}
