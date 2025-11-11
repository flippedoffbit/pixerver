package tasks

import (
	"errors"
	"time"

	"pixerver/logger"
	"pixerver/queue"
)

// QueueClient is a shared Redis Streams queue used by task helpers.
var QueueClient *queue.Queue

// CreateQueue opens (or creates) the stream and consumer group for tasks.
func CreateQueue(stream, group, consumer string) (*queue.Queue, error) {
	q, err := queue.New(stream, group, consumer)
	if err != nil {
		return nil, err
	}
	QueueClient = q
	return q, nil
}

// CloseQueue closes the underlying queue client.
func CloseQueue() error {
	if QueueClient == nil {
		return nil
	}
	return QueueClient.Close()
}

// TaskMessage is a simplified representation of a Redis Stream message.
type TaskMessage struct {
	ID     string
	Values map[string]interface{}
}

// Enqueue appends a job to the configured queue. Returns the message id.
func Enqueue(values map[string]interface{}) (string, error) {
	if QueueClient == nil {
		return "", ErrQueueNotOpen
	}
	return QueueClient.Produce(values)
}

// ReadNext reads messages from the queue for the configured consumer.
func ReadNext(block time.Duration, count int) ([]TaskMessage, error) {
	if QueueClient == nil {
		return nil, ErrQueueNotOpen
	}
	msgs, err := QueueClient.ReadNext(block, count)
	if err != nil {
		return nil, err
	}
	out := make([]TaskMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, TaskMessage{ID: m.ID, Values: m.Values})
	}
	return out, nil
}

// Ack acknowledges the provided message ids.
func Ack(ids ...string) error {
	if QueueClient == nil {
		return ErrQueueNotOpen
	}
	return QueueClient.Ack(ids...)
}

// Reclaim reclaims messages that have been idle for at least minIdle.
func Reclaim(minIdle time.Duration, count int) ([]TaskMessage, error) {
	if QueueClient == nil {
		return nil, ErrQueueNotOpen
	}
	msgs, err := QueueClient.Reclaim(minIdle, count)
	if err != nil {
		return nil, err
	}
	out := make([]TaskMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, TaskMessage{ID: m.ID, Values: m.Values})
	}
	if len(out) > 0 {
		logger.Infof("tasks: reclaimed %d messages", len(out))
	}
	return out, nil
}

// ErrQueueNotOpen is returned when the queue client hasn't been created.
var ErrQueueNotOpen = errors.New("queue not open")
