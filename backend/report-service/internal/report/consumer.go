package report

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	SubjectScoreComputed = "audit.events.score.computed"
	consumerName         = "report-service-worker"
)

type Consumer struct {
	svc    Service
	sub    *nats.Subscription
	logger *zap.Logger
}

func NewConsumer(js nats.JetStreamContext, svc Service, logger *zap.Logger) (*Consumer, error) {
	c := &Consumer{svc: svc, logger: logger}

	sub, err := js.Subscribe(
		SubjectScoreComputed,
		c.handleScoreComputed,
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
	logger.Info("report service NATS consumer started", zap.String("subject", SubjectScoreComputed))
	return c, nil
}

func (c *Consumer) Drain() {
	if c.sub != nil {
		_ = c.sub.Drain()
	}
}

func (c *Consumer) handleScoreComputed(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleScoreComputed(ctx, msg); err != nil {
		c.logger.Error("report generator failed", zap.Error(err))
		_ = msg.Ack()
		return
	}

	_ = msg.Ack()
}
