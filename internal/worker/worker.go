package worker

import (
	"context"
	"log"
	"time"
	"errors"
)

type Worker interface {
	Process(ctx context.Context) error
	Name() string
}

func Start(ctx context.Context, w Worker, period time.Duration) error {
	log.Printf("Воркер %s запущен", w.Name())
	if period == 0 {
		return errors.New("schedule period = 0")
	}

	go runWorkerPeriodically(ctx, w, period)

	return nil
}

// runWorkerPeriodically запускает воркер с периодом
func runWorkerPeriodically(ctx context.Context, w Worker, period time.Duration) {
	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("worker %s panic: %v", w.Name(), r)
					}
				}()


				if err := w.Process(ctx); err != nil {
					log.Printf("worker %s error: %v", w.Name(), err)
				}
			}()
		}
	}
}
