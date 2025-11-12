package queue

import (
	"testing"
	"time"
)

// Integration test for queue using Redis Streams. Skips if Redis not available.
func TestQueueProduceReadAck(t *testing.T) {
	q, err := New("test-stream", "test-group", "consumer-1")
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	defer q.Close()

	// produce a simple message
	id, err := q.Produce(map[string]interface{}{"k": "v"})
	if err != nil {
		t.Fatalf("Produce failed: %v", err)
	}
	if id == "" {
		t.Fatalf("Produce returned empty id")
	}

	// read the message (non-blocking)
	msgs, err := q.ReadNext(500*time.Millisecond, 10)
	if err != nil {
		t.Fatalf("ReadNext failed: %v", err)
	}
	if len(msgs) == 0 {
		t.Fatalf("expected at least one message")
	}

	// ack all read messages
	var ids []string
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}
	if err := q.Ack(ids...); err != nil {
		t.Fatalf("Ack failed: %v", err)
	}
}
