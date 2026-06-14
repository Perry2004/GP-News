package email

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

const utf8Charset = "UTF-8"

type Config struct {
	From   string
	To     []string
	Region string
}

type Sender struct {
	cfg    Config
	client sesClient
}

type sesClient interface {
	SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

func NewSESSender(ctx context.Context, cfg Config) (*Sender, error) {
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	loadOptions := []func(*config.LoadOptions) error{}
	if strings.TrimSpace(cfg.Region) != "" {
		loadOptions = append(loadOptions, config.WithRegion(strings.TrimSpace(cfg.Region)))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load AWS config for SES: %w", err)
	}

	return NewSender(cfg, sesv2.NewFromConfig(awsCfg))
}

func NewSender(cfg Config, client sesClient) (*Sender, error) {
	if client == nil {
		return nil, errors.New("email config: SES client is required")
	}
	if err := validateConfig(cfg); err != nil {
		return nil, err
	}

	cfg.From = strings.TrimSpace(cfg.From)
	cfg.Region = strings.TrimSpace(cfg.Region)
	cfg.To = normalizeRecipients(cfg.To)

	return &Sender{
		cfg:    cfg,
		client: client,
	}, nil
}

func (s *Sender) SendHTML(ctx context.Context, subject string, htmlBody string) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", errors.New("email subject is required")
	}
	if strings.TrimSpace(htmlBody) == "" {
		return "", errors.New("email HTML body is required")
	}

	output, err := s.client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(s.cfg.From),
		Destination: &types.Destination{
			ToAddresses: s.cfg.To,
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{
					Data:    aws.String(subject),
					Charset: aws.String(utf8Charset),
				},
				Body: &types.Body{
					Html: &types.Content{
						Data:    aws.String(htmlBody),
						Charset: aws.String(utf8Charset),
					},
				},
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("send SES email: %w", err)
	}
	if output == nil || output.MessageId == nil {
		return "", nil
	}

	return *output.MessageId, nil
}

func SendRenderedHTML(ctx context.Context, cfg Config, subject string, renderedEmailPath string) (string, error) {
	htmlBody, err := os.ReadFile(renderedEmailPath)
	if err != nil {
		return "", fmt.Errorf("read rendered email HTML %q: %w", renderedEmailPath, err)
	}

	sender, err := NewSESSender(ctx, cfg)
	if err != nil {
		return "", err
	}

	messageID, err := sender.SendHTML(ctx, subject, string(htmlBody))
	if err != nil {
		return "", err
	}

	slog.Info("Sent briefing email", "message_id", messageID, "recipient_count", len(normalizeRecipients(cfg.To)))
	return messageID, nil
}

func validateConfig(cfg Config) error {
	if strings.TrimSpace(cfg.From) == "" {
		return errors.New("email config: From is required")
	}
	if len(normalizeRecipients(cfg.To)) == 0 {
		return errors.New("email config: at least one To recipient is required")
	}
	return nil
}

func normalizeRecipients(recipients []string) []string {
	normalized := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		recipient = strings.TrimSpace(recipient)
		if recipient != "" {
			normalized = append(normalized, recipient)
		}
	}
	return normalized
}
