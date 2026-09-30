package orchestrator

import (
	"context"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

const (
	StreamName = "AUDIT_EVENTS"

	SubjectAuditValidated     = "audit.validated"
	SubjectStartCrawl         = "audit.commands.start_crawl"
	SubjectCrawlCompleted     = "audit.events.crawl_completed"
	SubjectCrawlFailed        = "audit.events.crawl_failed"
	SubjectParseCompleted     = "audit.events.parse_completed"
	SubjectParseFailed        = "audit.events.parse_failed"
	SubjectOnPageCompleted    = "audit.events.analysis.onpage.completed"
	SubjectTechnicalCompleted = "audit.events.analysis.technical.completed"
	SubjectKeywordsExtracted  = "audit.events.keywords.extracted"
	SubjectScoreComputed      = "audit.events.score.computed"
	SubjectReportGenerated    = "audit.events.report.generated"

	consumerName = "orchestrator"
)

type Consumer struct {
	svc    Service
	subs   []*nats.Subscription
	logger *zap.Logger
}

func NewConsumer(js nats.JetStreamContext, svc Service, logger *zap.Logger) (*Consumer, error) {
	c := &Consumer{svc: svc, logger: logger, subs: make([]*nats.Subscription, 0)}

	// 1. Subscribe to audit.validated
	sub1, err := js.Subscribe(
		SubjectAuditValidated,
		c.handleAuditValidated,
		nats.Durable(consumerName+"-validated"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub1)

	// 2. Subscribe to audit.events.crawl_completed
	sub2, err := js.Subscribe(
		SubjectCrawlCompleted,
		c.handleCrawlCompleted,
		nats.Durable(consumerName+"-crawl-completed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub2)

	// 3. Subscribe to audit.events.crawl_failed
	sub3, err := js.Subscribe(
		SubjectCrawlFailed,
		c.handleCrawlFailed,
		nats.Durable(consumerName+"-crawl-failed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub3)

	// 4. Subscribe to audit.events.parse_completed
	sub4, err := js.Subscribe(
		SubjectParseCompleted,
		c.handleParseCompleted,
		nats.Durable(consumerName+"-parse-completed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub4)

	// 5. Subscribe to audit.events.parse_failed
	sub5, err := js.Subscribe(
		SubjectParseFailed,
		c.handleParseFailed,
		nats.Durable(consumerName+"-parse-failed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub5)

	// 6. Subscribe to audit.events.analysis.onpage.completed
	sub6, err := js.Subscribe(
		SubjectOnPageCompleted,
		c.handleOnPageCompleted,
		nats.Durable(consumerName+"-onpage-completed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub6)

	// 7. Subscribe to audit.events.analysis.technical.completed
	sub7, err := js.Subscribe(
		SubjectTechnicalCompleted,
		c.handleTechnicalCompleted,
		nats.Durable(consumerName+"-technical-completed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub7)

	// 8. Subscribe to audit.events.keywords.extracted
	sub8, err := js.Subscribe(
		SubjectKeywordsExtracted,
		c.handleKeywordsExtracted,
		nats.Durable(consumerName+"-keywords-extracted"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub8)

	// 9. Subscribe to audit.events.score.computed
	sub9, err := js.Subscribe(
		SubjectScoreComputed,
		c.handleScoreComputed,
		nats.Durable(consumerName+"-score-computed"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub9)

	// 10. Subscribe to audit.events.report.generated
	sub10, err := js.Subscribe(
		SubjectReportGenerated,
		c.handleReportGenerated,
		nats.Durable(consumerName+"-report-generated"),
		nats.ManualAck(),
		nats.AckExplicit(),
		nats.MaxDeliver(5),
		nats.DeliverNew(),
	)
	if err != nil {
		return nil, err
	}
	c.subs = append(c.subs, sub10)

	logger.Info("NATS orchestrator consumer started for all audit lifecycle events (with 3-way parallel join and reporting)")
	return c, nil
}

func (c *Consumer) Drain() {
	for _, sub := range c.subs {
		if sub != nil {
			_ = sub.Drain()
		}
	}
}

//==========================================//
//             MESSAGE HANDLERS             //
//==========================================//

func (c *Consumer) handleAuditValidated(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleAuditValidated(ctx, msg); err != nil {
		c.logger.Error("audit.validated handler failed", zap.Error(err))
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *Consumer) handleCrawlCompleted(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleCrawlCompleted(ctx, msg); err != nil {
		c.logger.Error("crawl.completed handler failed", zap.Error(err))
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *Consumer) handleCrawlFailed(msg *nats.Msg) {
	ctx := context.Background()
	_ = c.svc.HandleCrawlFailed(ctx, msg)
	_ = msg.Ack()
}

func (c *Consumer) handleParseCompleted(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleParseCompleted(ctx, msg); err != nil {
		c.logger.Error("parse.completed handler failed", zap.Error(err))
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *Consumer) handleParseFailed(msg *nats.Msg) {
	ctx := context.Background()
	_ = c.svc.HandleParseFailed(ctx, msg)
	_ = msg.Ack()
}

func (c *Consumer) handleOnPageCompleted(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleOnPageAnalysisCompleted(ctx, msg); err != nil {
		c.logger.Error("onpage.completed handler failed", zap.Error(err))
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *Consumer) handleTechnicalCompleted(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleTechnicalAnalysisCompleted(ctx, msg); err != nil {
		c.logger.Error("technical.completed handler failed", zap.Error(err))
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *Consumer) handleKeywordsExtracted(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleKeywordsExtracted(ctx, msg); err != nil {
		c.logger.Error("keywords.extracted handler failed", zap.Error(err))
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *Consumer) handleScoreComputed(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleScoreComputed(ctx, msg); err != nil {
		c.logger.Error("score.computed handler failed", zap.Error(err))
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

func (c *Consumer) handleReportGenerated(msg *nats.Msg) {
	ctx := context.Background()
	if err := c.svc.HandleReportGenerated(ctx, msg); err != nil {
		c.logger.Error("report.generated handler failed", zap.Error(err))
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}
