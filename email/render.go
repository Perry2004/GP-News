package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/Perry2004/GP-News/briefing"
	emailtemplate "github.com/Perry2004/GP-News/email/template"
)

const (
	emailTemplatePath     = "email/template/out/template.html"
	renderedEmailFileName = "briefing_email.html"
)

func RenderBriefingHTML(briefingEmail briefing.BriefingEmail, cacheDir string) (string, error) {
	return renderBriefingHTMLWithBytes(
		briefingEmail,
		filepath.Base(emailTemplatePath),
		RenderedEmailFilePath(cacheDir),
		emailtemplate.HTML,
	)
}

func RenderedEmailFilePath(cacheDir string) string {
	return filepath.Join(normalizedCacheDir(cacheDir), renderedEmailFileName)
}

func normalizedCacheDir(cacheDir string) string {
	cacheDir = strings.TrimSpace(cacheDir)
	if cacheDir == "" {
		return "cache"
	}
	return cacheDir
}

func RenderBriefingHTMLWithPaths(briefingEmail briefing.BriefingEmail, templatePath string, outputPath string) (string, error) {
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return "", fmt.Errorf("read email template %q: %w", templatePath, err)
	}
	return renderBriefingHTMLWithBytes(briefingEmail, filepath.Base(templatePath), outputPath, templateBytes)
}

func renderBriefingHTMLWithBytes(briefingEmail briefing.BriefingEmail, templateName string, outputPath string, templateBytes []byte) (string, error) {
	data, err := BriefingTemplateData(briefingEmail)
	if err != nil {
		return "", err
	}
	if err := executeHTMLTemplateBytes(templateName, outputPath, templateBytes, data); err != nil {
		return "", err
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		return "", fmt.Errorf("stat rendered email HTML: %w", err)
	}
	slog.Info("Briefing email HTML written", "file", outputPath, "bytes", info.Size())
	return outputPath, nil
}

func BriefingTemplateData(briefingEmail briefing.BriefingEmail) (map[string]any, error) {
	payload, err := json.Marshal(briefingEmail)
	if err != nil {
		return nil, fmt.Errorf("marshal briefing email: %w", err)
	}
	var data map[string]any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("unmarshal briefing email template data: %w", err)
	}
	data["full_news_card_count"] = len(briefingEmail.TopNewsByTopic.MarketsMacro) +
		len(briefingEmail.TopNewsByTopic.PoliticsPolicy) +
		len(briefingEmail.TopNewsByTopic.WarGeopoliticalRisk) +
		len(briefingEmail.TopNewsByTopic.TechnologyAI)
	return data, nil
}

func ExecuteHTMLTemplate(templatePath string, outputPath string, data any) error {
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("read email template %q: %w", templatePath, err)
	}
	return executeHTMLTemplateBytes(filepath.Base(templatePath), outputPath, templateBytes, data)
}

func executeHTMLTemplateBytes(templateName string, outputPath string, templateBytes []byte, data any) error {
	tmpl, err := htmltemplate.New(templateName).Funcs(htmltemplate.FuncMap{
		"inc": func(value int) int {
			return value + 1
		},
	}).Parse(string(templateBytes))
	if err != nil {
		return fmt.Errorf("parse email template %q: %w", templateName, err)
	}

	var output bytes.Buffer
	if err := tmpl.Execute(&output, data); err != nil {
		return fmt.Errorf("execute email template %q: %w", templateName, err)
	}
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("create rendered email directory: %w", err)
	}
	if err := os.WriteFile(outputPath, output.Bytes(), 0644); err != nil {
		return fmt.Errorf("write rendered email HTML %q: %w", outputPath, err)
	}
	return nil
}
