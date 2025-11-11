package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"pixerver/logger"
	"pixerver/queue"
)

func main() {
	consumer := "consumer-1"
	if len(os.Args) > 1 {
		consumer = os.Args[1]
	}
	q, err := queue.New("jobs", "workers", consumer)
	if err != nil {
		logger.Errorf("consumer: %v", err)
		os.Exit(1)
	}
	defer q.Close()

	// handle SIGTERM to exit gracefully
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		fmt.Println("shutting down")
		q.Close()
		os.Exit(0)
	}()

	for {
		msgs, err := q.ReadNext(5*time.Second, 10)
		if err != nil {
			logger.Errorf("consumer: read failed: %v", err)
			time.Sleep(time.Second)
			continue
		}
		if len(msgs) == 0 {
			// nothing available; try reclaiming stale pending messages
			reclaimed, err := q.Reclaim(30*time.Second, 10)
			if err != nil {
				logger.Errorf("consumer: reclaim failed: %v", err)
			} else if len(reclaimed) > 0 {
				logger.Infof("consumer: reclaimed %d messages", len(reclaimed))
				// ack reclaimed messages to mark them handled in this example
				var ids []string
				for _, m := range reclaimed {
					ids = append(ids, m.ID)
				}
				if err := q.Ack(ids...); err != nil {
					logger.Errorf("consumer: ack failed: %v", err)
				}
			}
			continue
		}
		for _, m := range msgs {
			logger.Infof("consumer: got id=%s values=%v", m.ID, m.Values)
			// in a real worker you'd process the job here; we ack immediately
			if err := q.Ack(m.ID); err != nil {
				logger.Errorf("consumer: ack failed: %v", err)
			}
		}
	}
}
