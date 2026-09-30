package natsplatform

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Client struct {
	Conn *nats.Conn
	JS   nats.JetStreamContext
}

func Connect(natsURL string, logger *zap.Logger) (*Client, error) {
	nc, err := nats.Connect(
		natsURL,
		nats.MaxReconnects(-1),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			if err != nil {
				logger.Warn("NATS disconnected", zap.Error(err))
			}
		}),
		nats.ReconnectHandler(func(c *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", c.ConnectedUrl()))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS (%s): %w", natsURL, err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	// Ensure AUDIT stream exists
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "AUDIT",
		Subjects: []string{"audit.events.>", "audit.commands.>"},
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		logger.Warn("could not ensure AUDIT stream (may already exist)", zap.Error(err))
	}

	logger.Info("NATS connected and JetStream ready", zap.String("url", natsURL))
	return &Client{Conn: nc, JS: js}, nil
}

func (c *Client) Drain() {
	if c.Conn != nil {
		_ = c.Conn.Drain()
	}
}
