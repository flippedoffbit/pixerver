package uploads

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"pixerver/logger"

	"github.com/redis/go-redis/v9"
)

type StreamQueue interface {
	Producer
	ReadNext(block time.Duration, count int) ([]redis.XMessage, error)
	Ack(ids ...string) error
}

type Worker struct {
	Service *Service
	Queue   StreamQueue
	Block   time.Duration
}

func (w Worker) Run(ctx context.Context) {
	block := w.Block
	if block == 0 {
		block = time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		msgs, err := w.Queue.ReadNext(block, 1)
		if err != nil {
			logger.Warnf("worker: read queue failed: %v", err)
			continue
		}
		for _, msg := range msgs {
			w.handleMessage(ctx, msg)
		}
	}
}

func (w Worker) handleMessage(ctx context.Context, msg redis.XMessage) {
	uploadID, jobID, err := queueIDs(msg.Values)
	if err != nil {
		logger.Warnf("worker: invalid queue message %s: %v", msg.ID, err)
		_ = w.Queue.Ack(msg.ID)
		return
	}
	if err := w.Service.ProcessJob(ctx, uploadID, jobID); err != nil {
		logger.Warnf("worker: job %s upload %s failed: %v", jobID, uploadID, err)
		if job, getErr := w.Service.Repo.GetJob(jobID); getErr == nil && job.Status == JobStatusQueued {
			if _, produceErr := w.Queue.Produce(map[string]interface{}{"uploadId": uploadID, "jobId": jobID}); produceErr != nil {
				logger.Warnf("worker: requeue job %s failed: %v", jobID, produceErr)
			}
		}
	}
	if err := w.Queue.Ack(msg.ID); err != nil {
		logger.Warnf("worker: ack %s failed: %v", msg.ID, err)
	}
}

func queueIDs(values map[string]interface{}) (string, string, error) {
	uploadID := stringValue(values["uploadId"])
	jobID := stringValue(values["jobId"])
	if uploadID == "" || jobID == "" {
		return "", "", fmt.Errorf("missing uploadId or jobId")
	}
	return uploadID, jobID, nil
}

func stringValue(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case fmt.Stringer:
		return t.String()
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return ""
	}
}
