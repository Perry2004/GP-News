package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/caarlos0/env/v11"

	"github.com/Perry2004/GP-News/briefing"
	"github.com/Perry2004/GP-News/email"
)

// Final returned result
type Result struct {
	Subject   string `json:"subject,omitempty"`
	EmailSent bool   `json:"email_sent"`
	MessageID string `json:"message_id,omitempty"`
}

func Run(ctx context.Context) (Result, error) {
	envName, err := loadEnv()
	if err != nil {
		return Result{}, err
	}
	cfg, err := env.ParseAs[config]()
	if err != nil {
		return Result{}, err
	}
	if err := validateConfig(cfg); err != nil {
		return Result{}, err
	}

	if err := configureLogger(cfg); err != nil {
		return Result{}, err
	}

	slog.Debug("GP-News configuration loaded", "environment", envName, "config", maskedConfig(cfg))
	slog.Info("Starting GP-News")

	freshFrom, err := freshFromConfig(cfg)
	if err != nil {
		return Result{}, err
	}

	briefingEmail, err := generateBriefingEmail(ctx, cfg, freshFrom)
	if err != nil {
		return Result{}, err
	}

	renderedEmailPath, err := email.RenderBriefingHTML(briefingEmail, cfg.CacheDir)
	if err != nil {
		return Result{}, fmt.Errorf("failed to render briefing email: %w", err)
	}
	slog.Info("Rendered briefing email", "file", renderedEmailPath)

	var messageID string
	if cfg.EnableEmailSending {
		messageID, err = email.SendRenderedHTML(ctx, email.Config{
			From:   cfg.EmailFrom,
			To:     cfg.EmailTo,
			Region: cfg.AWSSESRegion,
		}, briefingEmail.Subject, renderedEmailPath)
		if err != nil {
			return Result{}, fmt.Errorf("failed to send briefing email: %w", err)
		}
	} else {
		slog.Info("Email sending disabled", "enable_email_sending", cfg.EnableEmailSending)
	}

	return resultForBriefing(cfg, briefingEmail, messageID), nil
}

func resultForBriefing(cfg config, briefingEmail briefing.BriefingEmail, messageID string) Result {
	return Result{
		Subject:   briefingEmail.Subject,
		EmailSent: cfg.EnableEmailSending,
		MessageID: messageID,
	}
}
