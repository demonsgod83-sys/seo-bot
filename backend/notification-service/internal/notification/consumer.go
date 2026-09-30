package notification

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectReportGenerated = "audit.events.report.generated"
	SubjectCrawlFailed     = "audit.events.crawl_failed"
	SubjectParseFailed     = "audit.events.parse_failed"

	consumerName = "notification-service-worker"
)

type Consumer struct {
	svc    Service
	subs   []*nats.Subscription
	logger *zap.Logger
}

func NewConsumer(js nats.JetStreamContext, svc Service, logger *zap.Logger) (*Consumer, error) {
	c := &Consumer{svc: svc, logger: logger, subs: make([]*nats.Subscription, 0)}

	// 1. Subscribe to report.generated
	sub1, err := js.Subscribe(
		SubjectReportGenerated,
		c.handleReportGenerated,
		nats.Durable(consumerName+"-report-generated"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(3),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub1)

	// 2. Subscribe to crawl_failed
	sub2, err := js.Subscribe(
		SubjectCrawlFailed,
		c.handleAuditFailed,
		nats.Durable(consumerName+"-crawl-failed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(3),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub2)

	// 3. Subscribe to parse_failed
	sub3, err := js.Subscribe(
		SubjectParseFailed,
		c.handleAuditFailed,
		nats.Durable(consumerName+"-parse-failed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(3),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub3)

	logger.Info("notification service NATS consumer started")
	return c, nil
}

func (c *Consumer) Drain() {
	for _, sub := range c.subs {
		if sub != nil {
			_ = sub.Drain()
		}
	}
}

func (c *Consumer) handleReportGenerated(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleReportGenerated(ctx, msg); err != nil {
		c.logger.Error("handleReportGenerated failed", zap.Error(err))
		_ = msg.Ack()
		return
	}
	_ = msg.Ack()
}

func (c *Consumer) handleAuditFailed(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleAuditFailed(ctx, msg); err != nil {
		c.logger.Error("handleAuditFailed failed", zap.Error(err))
		_ = msg.Ack()
		return
	}
	_ = msg.Ack()
}
