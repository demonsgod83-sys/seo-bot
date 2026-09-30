package auditor

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectParseCompleted = "audit.events.parse_completed"
	consumerName          = "technical-auditor-worker"
)

type Consumer struct {
	svc    Service
	sub    *nats.Subscription
	logger *zap.Logger
}

func NewConsumer(js nats.JetStreamContext, svc Service, logger *zap.Logger) (*Consumer, error) {
	c := &Consumer{svc: svc, logger: logger}

	sub, err := js.Subscribe(
		SubjectParseCompleted,
		c.handleParseCompleted,
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
	logger.Info("technical auditor NATS consumer started", zap.String("subject", SubjectParseCompleted))
	return c, nil
}

func (c *Consumer) Drain() {
	if c.sub != nil {
		_ = c.sub.Drain()
	}
}

func (c *Consumer) handleParseCompleted(msg *nats.Msg) {
	var event ParseCompletedEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		c.logger.Error("malformed parse_completed event — discarding", zap.Error(err))
		_ = msg.Ack()
		return
	}

	c.logger.Info("parse_completed received by technical auditor",
		zap.String("audit_id", event.AuditID.String()),
		zap.String("domain", event.Domain),
	)

	ctx := context.Background()
	if err := c.svc.ExecuteAudit(ctx, event); err != nil {
		c.logger.Warn("technical audit execution encountered error",
			zap.String("audit_id", event.AuditID.String()),
			zap.Error(err),
		)
		_ = msg.Ack()
		return
	}

	_ = msg.Ack()
}
