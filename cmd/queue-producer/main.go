package main

import (
	"fmt"
	"os"
	"time"

	"pixerver/logger"
	"pixerver/queue"
)

func main() {
	msg := "hello"
	if len(os.Args) > 1 {
		msg = os.Args[1]
	}
	q, err := queue.New("jobs", "workers", "producer-1")
	if err != nil {
		logger.Errorf("producer: %v", err)
		os.Exit(1)
	}
	defer q.Close()
	id, err := q.Produce(map[string]interface{}{"payload": msg, "ts": time.Now().UTC().Format(time.RFC3339)})
	if err != nil {
		logger.Errorf("producer: produce failed: %v", err)
		os.Exit(1)
	}
	fmt.Printf("produced id=%s\n", id)
}
