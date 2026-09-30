package scoring

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectAnalyzingCompleted = "audit.events.analyzing.completed"
	consumerName              = "scoring-service-worker"
)

type Consumer struct {
	svc    Service
	sub    *nats.Subscription
	logger *zap.Logger
}

func NewConsumer(js nats.JetStreamContext, svc Service, logger *zap.Logger) (*Consumer, error) {
	c := &Consumer{svc: svc, logger: logger}

	sub, err := js.Subscribe(
		SubjectAnalyzingCompleted,
		c.handleAnalyzingCompleted,
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
	logger.Info("scoring service NATS consumer started", zap.String("subject", SubjectAnalyzingCompleted))
	return c, nil
}

func (c *Consumer) Drain() {
	if c.sub != nil {
		_ = c.sub.Drain()
	}
}

func (c *Consumer) handleAnalyzingCompleted(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleStartScoring(ctx, msg); err != nil {
		c.logger.Error("scoring handler failed", zap.Error(err))
		// Ack or let retry depending on policy
		_ = msg.Ack()
		return
	}

	_ = msg.Ack()
}
