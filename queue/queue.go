package queue

import (
	"context"
	"fmt"
	"strings"
	"time"

	"pixerver/internal/redisclient"
	"pixerver/logger"

	"github.com/redis/go-redis/v9"
)

// Queue is a small helper around Redis Streams for producing and consuming jobs.
type Queue struct {
	client   *redis.Client
	stream   string
	group    string
	consumer string
}

// New creates or connects to a stream and consumer group. If the group already
// exists, the BUSYGROUP error is ignored.
func New(stream, group, consumer string) (*Queue, error) {
	client, err := redisclient.NewClient()
	if err != nil {
		return nil, err
	}
	q := &Queue{client: client, stream: stream, group: group, consumer: consumer}
	ctx := context.Background()
	// create group; use MKSTREAM so stream is created if missing
	if err := client.XGroupCreateMkStream(ctx, stream, group, "0").Err(); err != nil {
		// when group already exists, Redis returns BUSYGROUP; ignore it
		if !strings.Contains(err.Error(), "BUSYGROUP") {
			logger.Errorf("queue: failed to create group: %v", err)
			return nil, err
		}
	}
	logger.Infof("queue: ready stream=%s group=%s consumer=%s", stream, group, consumer)
	return q, nil
}

// Close closes the underlying Redis client.
func (q *Queue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}

// Produce appends a message to the stream. Values is a map of string->interface{}.
// Returns the message ID on success.
func (q *Queue) Produce(values map[string]interface{}) (string, error) {
	if q == nil || q.client == nil {
		return "", fmt.Errorf("queue: not initialized")
	}
	ctx := context.Background()
	id, err := q.client.XAdd(ctx, &redis.XAddArgs{Stream: q.stream, Values: values}).Result()
	if err != nil {
		return "", err
	}
	return id, nil
}

// ReadNext reads messages for this consumer using XREADGROUP. Block indicates the
// maximum blocking duration; use 0 for no blocking. Count limits number of messages.
func (q *Queue) ReadNext(block time.Duration, count int) ([]redis.XMessage, error) {
	if q == nil || q.client == nil {
		return nil, fmt.Errorf("queue: not initialized")
	}
	ctx := context.Background()
	args := &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: q.consumer,
		Streams:  []string{q.stream, ">"},
		Count:    int64(count),
		Block:    block,
	}
	res, err := q.client.XReadGroup(ctx, args).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var out []redis.XMessage
	for _, st := range res {
		out = append(out, st.Messages...)
	}
	return out, nil
}

// Ack acknowledges the given message IDs so they are removed from the pending list.
func (q *Queue) Ack(ids ...string) error {
	if q == nil || q.client == nil {
		return fmt.Errorf("queue: not initialized")
	}
	ctx := context.Background()
	if _, err := q.client.XAck(ctx, q.stream, q.group, ids...).Result(); err != nil {
		return err
	}
	return nil
}

// Reclaim attempts to claim pending messages that have been idle for at least
// minIdle and returns the claimed messages. It uses XAUTOCLAIM under the hood.
// count limits how many messages to reclaim in one call.
func (q *Queue) Reclaim(minIdle time.Duration, count int) ([]redis.XMessage, error) {
	if q == nil || q.client == nil {
		return nil, fmt.Errorf("queue: not initialized")
	}
	ctx := context.Background()
	// start cursor at 0 to scan entire PEL
	msgs, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   q.stream,
		Group:    q.group,
		Consumer: q.consumer,
		MinIdle:  minIdle,
		Start:    "0",
		Count:    int64(count),
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	return msgs, nil
}
