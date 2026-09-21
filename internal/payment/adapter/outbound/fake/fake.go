package fake

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	mu      sync.Mutex
	created map[string]time.Time
	delay   time.Duration
}

func NewClient(delay time.Duration) *Client {
	return &Client{
		created: make(map[string]time.Time),
		delay:   delay,
	}
}

func (c *Client) CreateInvoice(
	_ context.Context,
	_ string,
	_ int,
) (string, string, error) {
	id := uuid.NewString()

	c.mu.Lock()
	c.created[id] = time.Now()
	c.mu.Unlock()

	return id, "https://example.com/fake-pay/" + id, nil
}

func (c *Client) CheckStatus(_ context.Context, invoiceID string) (bool, error) {
	c.mu.Lock()
	started, ok := c.created[invoiceID]
	c.mu.Unlock()

	if !ok {
		return false, fmt.Errorf("fake: неизвестный инвойс %s", invoiceID)
	}

	return time.Since(started) >= c.delay, nil
}
