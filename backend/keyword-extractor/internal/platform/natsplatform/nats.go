package natsplatform

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

type Client struct {
	Conn *nats.Conn
	JS   nats.JetStreamContext
}

func Connect(url string, logger *zap.Logger) (*Client, error) {
	nc, err := nats.Connect(
		url,
		nats.Name("seo-bot-keyword-extractor"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.Warn("NATS disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to NATS (%s): %w", url, err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("failed to create JetStream context: %w", err)
	}

	if err := ensureStream(js, logger); err != nil {
		nc.Close()
		return nil, err
	}

	logger.Info("NATS connected and stream ready", zap.String("url", url))
	return &Client{Conn: nc, JS: js}, nil
}

func (c *Client) Drain() {
	if c.Conn != nil {
		c.Conn.Drain()
	}
}

func ensureStream(js nats.JetStreamContext, logger *zap.Logger) error {
	const streamName = "AUDIT_EVENTS"

	_, err := js.StreamInfo(streamName)
	if err == nats.ErrStreamNotFound {
		_, err = js.AddStream(&nats.StreamConfig{
			Name:       streamName,
			Subjects:   []string{"audit.>"},
			Storage:    nats.FileStorage,
			Replicas:   1,
			Retention:  nats.LimitsPolicy,
			MaxAge:     24 * time.Hour,
			Duplicates: 5 * time.Minute,
		})
		if err != nil {
			return fmt.Errorf("failed to create NATS stream: %w", err)
		}
		logger.Info("NATS stream created", zap.String("stream", streamName))
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check NATS stream: %w", err)
	}

	logger.Info("NATS stream already exists", zap.String("stream", streamName))
	return nil
}
