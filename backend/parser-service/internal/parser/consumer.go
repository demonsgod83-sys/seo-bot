package parser

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectCrawlCompleted = "audit.events.crawl_completed"
	consumerName          = "parser-worker"
)

type Consumer struct {
	svc    Service
	sub    *nats.Subscription
	logger *zap.Logger
}

func NewConsumer(js nats.JetStreamContext, svc Service, logger *zap.Logger) (*Consumer, error) {
	c := &Consumer{svc: svc, logger: logger}

	sub, err := js.Subscribe(
		SubjectCrawlCompleted,
		c.handleCrawlCompleted,
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
	logger.Info("parser NATS consumer started", zap.String("subject", SubjectCrawlCompleted))
	return c, nil
}

func (c *Consumer) Drain() {
	if c.sub != nil {
		_ = c.sub.Drain()
	}
}

func (c *Consumer) handleCrawlCompleted(msg *nats.Msg) {
	var event CrawlCompletedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		c.logger.Error("malformed crawl_completed event — discarding", zap.Error(err))
		_ = msg.Ack()
		return
	}

	c.logger.Info("crawl_completed received by parser",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	ctx := context.Background()
	if err := c.svc.ExecuteParse(ctx, event); err != nil {
		c.logger.Warn("parse execution encountered error",
			zap.String("audit_id", event.AuditID.String()),
			zap.Error(err),
		)
		_ = msg.Ack()
		return
	}

	_ = msg.Ack()
}
