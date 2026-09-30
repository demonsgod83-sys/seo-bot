package notification

import (
	"fmt"
	"net/smtp"
	"strings"

	"go.uber.org/zap"
)

type EmailSender interface {
	SendAuditCompletedEmail(
		to string,
		domain string,
		overallScore int,
		technicalScore, onpageScore, contentScore int,
		topIssues []PrioritizedIssue,
		shareURL string,
	) error

	SendAuditFailedEmail(to, domain, reason string) error
}

type smtpEmailSender struct {
	enabled  bool
	host     string
	port     int
	user     string
	pass     string
	from     string
	logger   *zap.Logger
}

func NewEmailSender(
	enabled bool,
	host string,
	port int,
	user, pass, from string,
	logger *zap.Logger,
) EmailSender {
	return &smtpEmailSender{
		enabled:  enabled,
		host:     host,
		port:     port,
		user:     user,
		pass:     pass,
		from:     from,
		logger:   logger,
	}
}

func (s *smtpEmailSender) SendAuditCompletedEmail(
	to string,
	domain string,
	overallScore int,
	technicalScore, onpageScore, contentScore int,
	topIssues []PrioritizedIssue,
	shareURL string,
) error {
	subject := fmt.Sprintf("SEO Audit Complete for %s (Score: %d/100)", domain, overallScore)

	var issuesList strings.Builder
	for i, issue := range topIssues {
		if i >= 3 {
			break
		}
		issuesList.WriteString(fmt.Sprintf("  %d. %s [%s] — %s (Priority: %.1f, %d pages affected)\n",
			i+1, issue.Title, strings.ToUpper(issue.Severity), issue.EffortTier+" effort", issue.PriorityScore, issue.AffectedPagesCount))
	}
	if issuesList.Len() == 0 {
		issuesList.WriteString("  No critical issues found! Clean audit report.\n")
	}

	body := fmt.Sprintf(
`Hello,

Your SEO audit for %s has completed successfully.

=== EXECUTIVE SUMMARY ===
Overall SEO Score: %d/100
- Technical Health: %d/100
- On-Page Quality: %d/100
- Content & Keywords: %d/100

=== TOP ACTIONABLE ISSUES ===
%s
View Full Interactive Web Report & Download PDF:
%s

Best regards,
SEO-Bot Team
`, domain, overallScore, technicalScore, onpageScore, contentScore, issuesList.String(), shareURL)

	if !s.enabled {
		s.logger.Info("📧 [EMAIL MOCK DELIVERED]",
			zap.String("to", to),
			zap.String("subject", subject),
			zap.Int("overall_score", overallScore),
			zap.String("share_url", shareURL),
		)
		return nil
	}

	return s.sendSMTP(to, subject, body)
}

func (s *smtpEmailSender) SendAuditFailedEmail(to, domain, reason string) error {
	subject := fmt.Sprintf("SEO Audit Failed for %s", domain)
	body := fmt.Sprintf(
`Hello,

Unfortunately, your SEO audit for %s could not be completed.

Failure Reason:
%s

Please check that the domain is reachable and try submitting again.

Best regards,
SEO-Bot Team
`, domain, reason)

	if !s.enabled {
		s.logger.Info("📧 [EMAIL MOCK DELIVERED (AUDIT FAILED)]",
			zap.String("to", to),
			zap.String("subject", subject),
			zap.String("reason", reason),
		)
		return nil
	}

	return s.sendSMTP(to, subject, body)
}

func (s *smtpEmailSender) sendSMTP(to, subject, body string) error {
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", s.from, to, subject, body)
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	var auth smtp.Auth
	if s.user != "" && s.pass != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}

	err := smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg))
	if err != nil {
		s.logger.Error("SMTP delivery failed", zap.String("to", to), zap.Error(err))
		return fmt.Errorf("smtp send: %w", err)
	}

	s.logger.Info("email delivered via SMTP", zap.String("to", to))
	return nil
}
