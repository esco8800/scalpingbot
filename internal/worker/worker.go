package worker

import (
	"context"
	"errors"
	"fmt"
	"log"
	"scalpingbot/internal/logger"
	"scalpingbot/internal/tools"
	"time"
)

type Worker interface {
	Process(ctx context.Context) error
	Name() string
}

func Start(ctx context.Context, w Worker, period time.Duration, logLogger logger.Logger) error {
	log.Printf("Воркер %s запущен", w.Name())
	if period == 0 {
		return errors.New("schedule period = 0")
	}

	go runWorkerPeriodically(ctx, w, period, logLogger)

	return nil
}

// runWorkerPeriodically запускает воркер с периодом
func runWorkerPeriodically(ctx context.Context, w Worker, period time.Duration, logLogger logger.Logger) {
	defer func() {
		if r := recover(); r != nil {
			log.Print(ctx, "Worker-Panic", fmt.Errorf("%v", r), "worker name ", w.Name())
			go runWorkerPeriodically(ctx, w, period, logLogger)
		}
	}()

	for {
		var err error

		select {
		case <-ctx.Done():
			return
		default:
			err = w.Process(ctx)
		}

		if err != nil {
			tools.LogErrorf("Worker-Error name: %s, err: %v", err, w.Name())
			logLogger.Error(fmt.Sprintf("Worker-Error name: %s, err: %v", err, w.Name()))
			time.Sleep(period)
		} else {
			log.Print("Worker job success", "worker name ", w.Name())
			time.Sleep(period)
		}
	}
}
