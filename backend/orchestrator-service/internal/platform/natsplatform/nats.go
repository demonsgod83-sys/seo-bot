package natsplatform

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

// Client holds the raw NATS connection and the JetStream context.
// Both are exported so they can be injected into other services.
type Client struct {
	Conn *nats.Conn
	JS   nats.JetStreamContext
}

// Connect creates a NATS connection and initialises JetStream.
// It also ensures the AUDIT_EVENTS stream exists (idempotent — safe to call
// on every startup even if the stream was already created by another instance).
func Connect(url string, logger *zap.Logger) (*Client, error) {
	nc, err := nats.Connect(
		url,
		nats.Name("seo-bot-orchestrator"),
		nats.MaxReconnects(-1), // reconnect forever
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			logger.Warn("NATS disconnected", zap.Error(err))
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info("NATS reconnected", zap.String("url", nc.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(nc *nats.Conn) {
			logger.Info("NATS connection closed")
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

// Drain gracefully drains the connection — waits for all published messages
// to be acknowledged before closing. Call this during graceful shutdown.
func (c *Client) Drain() {
	if c.Conn != nil {
		c.Conn.Drain()
	}
}

//==========================================//
//             STREAM SETUP                 //
//==========================================//

// ensureStream creates the AUDIT_EVENTS JetStream stream if it doesn't exist.
// If it already exists, it's a no-op.
func ensureStream(js nats.JetStreamContext, logger *zap.Logger) error {
	cfg := &nats.StreamConfig{
		Name: "AUDIT_EVENTS",
		// Wildcard — all audit lifecycle events land here.
		// New event types just need a new subject; no stream change required.
		Subjects: []string{"audit.>"},

		Storage:  nats.FileStorage, // survives NATS server restarts
		Replicas: 1,                // increase to 3 in a clustered prod deployment
		Retention: nats.LimitsPolicy,

		// Keep messages for 24 h — long enough for any consumer to catch up
		// after a short outage without blowing up disk.
		MaxAge: 24 * time.Hour,

		// Deduplication window for Nats-Msg-Id header.
		// If a publisher retries within 5 min, JetStream drops the duplicate.
		Duplicates: 5 * time.Minute,
	}

	_, err := js.StreamInfo(cfg.Name)
	if err == nats.ErrStreamNotFound {
		if _, err := js.AddStream(cfg); err != nil {
			return fmt.Errorf("failed to create NATS stream %q: %w", cfg.Name, err)
		}
		logger.Info("NATS stream created", zap.String("stream", cfg.Name))
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to check NATS stream %q: %w", cfg.Name, err)
	}

	logger.Info("NATS stream already exists", zap.String("stream", cfg.Name))
	return nil
}
