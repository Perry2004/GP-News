package email

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
)

type fakeSESClient struct {
	input *sesv2.SendEmailInput
	err   error
}

func (f *fakeSESClient) SendEmail(ctx context.Context, params *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	f.input = params
	if f.err != nil {
		return nil, f.err
	}
	return &sesv2.SendEmailOutput{MessageId: aws.String("ses-message-id")}, nil
}

func TestSendHTMLBuildsSESSimpleHTMLMessage(t *testing.T) {
	client := &fakeSESClient{}
	sender, err := NewSender(Config{
		From:   " sender@example.com ",
		To:     []string{" reader@example.com ", "", "desk@example.com"},
		Region: " us-west-2 ",
	}, client)
	if err != nil {
		t.Fatalf("NewSender() error = %v", err)
	}

	messageID, err := sender.SendHTML(context.Background(), "Briefing subject", "<html>briefing</html>")
	if err != nil {
		t.Fatalf("SendHTML() error = %v", err)
	}
	if messageID != "ses-message-id" {
		t.Fatalf("message ID = %q, want ses-message-id", messageID)
	}

	input := client.input
	if input == nil {
		t.Fatal("SendEmail was not called")
	}
	if got := aws.ToString(input.FromEmailAddress); got != "sender@example.com" {
		t.Fatalf("FromEmailAddress = %q, want sender@example.com", got)
	}
	if got := input.Destination.ToAddresses; len(got) != 2 || got[0] != "reader@example.com" || got[1] != "desk@example.com" {
		t.Fatalf("ToAddresses = %#v, want [reader@example.com desk@example.com]", got)
	}
	if input.Content == nil || input.Content.Simple == nil {
		t.Fatalf("Content.Simple missing: %#v", input.Content)
	}
	message := input.Content.Simple
	if got := aws.ToString(message.Subject.Data); got != "Briefing subject" {
		t.Fatalf("Subject.Data = %q, want Briefing subject", got)
	}
	if got := aws.ToString(message.Subject.Charset); got != utf8Charset {
		t.Fatalf("Subject.Charset = %q, want %s", got, utf8Charset)
	}
	if message.Body == nil || message.Body.Html == nil {
		t.Fatalf("HTML body missing: %#v", message.Body)
	}
	if got := aws.ToString(message.Body.Html.Data); got != "<html>briefing</html>" {
		t.Fatalf("Body.Html.Data = %q, want rendered HTML", got)
	}
	if got := aws.ToString(message.Body.Html.Charset); got != utf8Charset {
		t.Fatalf("Body.Html.Charset = %q, want %s", got, utf8Charset)
	}
	if message.Body.Text != nil {
		t.Fatalf("Body.Text = %#v, want nil", message.Body.Text)
	}
}

func TestNewSenderRequiresConfig(t *testing.T) {
	client := &fakeSESClient{}
	tests := []struct {
		name string
		cfg  Config
		want string
	}{
		{
			name: "from",
			cfg:  Config{To: []string{"reader@example.com"}},
			want: "From is required",
		},
		{
			name: "to",
			cfg:  Config{From: "sender@example.com", To: []string{"", " "}},
			want: "at least one To recipient is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewSender(tt.cfg, client)
			if err == nil {
				t.Fatal("NewSender() returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestSendHTMLWrapsSESError(t *testing.T) {
	sender, err := NewSender(Config{
		From: "sender@example.com",
		To:   []string{"reader@example.com"},
	}, &fakeSESClient{err: errors.New("boom")})
	if err != nil {
		t.Fatalf("NewSender() error = %v", err)
	}

	_, err = sender.SendHTML(context.Background(), "Briefing subject", "<html>briefing</html>")
	if err == nil {
		t.Fatal("SendHTML() returned nil error")
	}
	if !strings.Contains(err.Error(), "send SES email") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %q, want wrapped SES error", err.Error())
	}
}
