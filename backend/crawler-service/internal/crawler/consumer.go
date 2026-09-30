package crawler

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectStartCrawl = "audit.commands.start_crawl"
	consumerName      = "crawler-worker"
)

type Consumer struct {
	svc    Service
	sub    *nats.Subscription
	logger *zap.Logger
}

func NewConsumer(js nats.JetStreamContext, svc Service, logger *zap.Logger) (*Consumer, error) {
	c := &Consumer{svc: svc, logger: logger}

	sub, err := js.Subscribe(
		SubjectStartCrawl,
		c.handleStartCrawl,
		nats.Durable(consumerName),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(3),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}

	c.sub = sub
	logger.Info("crawler NATS consumer started", zap.String("subject", SubjectStartCrawl))
	return c, nil
}

func (c *Consumer) Drain() {
	if c.sub != nil {
		_ = c.sub.Drain()
	}
}

func (c *Consumer) handleStartCrawl(msg *nats.Msg) {
	var cmd StartCrawlCommand
	if err := json.Unmarshal(msg.Data, &cmd); err != nil {
		c.logger.Error("malformed start_crawl command — discarding", zap.Error(err))
		_ = msg.Ack()
		return
	}

	c.logger.Info("start_crawl received",
		zap.String("audit_id", cmd.AuditID.String()),
		zap.String("domain", cmd.Domain),
	)

	// Execute the crawl job in a background context with cancellation
	ctx := context.Background()
	if err := c.svc.ExecuteCrawl(ctx, cmd); err != nil {
		c.logger.Warn("crawl execution encountered error",
			zap.String("audit_id", cmd.AuditID.String()),
			zap.Error(err),
		)
		// We ack because crawl failures are published as explicit failure events rather than redelivering infinitely
		_ = msg.Ack()
		return
	}

	_ = msg.Ack()
}
