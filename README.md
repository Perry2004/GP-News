# GP-News

## Data Sources
- Yahoo Finance API for market values
   - Uses Yahoo finance's unofficial Chart API from https://github.com/ranaroussi/yfinance
   - Fetches 5-day daily chart data so market inputs include current level, recent closes, and computed daily change when enough history is available.
- NewsData.io API for news articles
   - It is only used for fetching news articles without actual content to avoid any subscription requirements. Only a registered account and an API key are needed.

## Email Rendering
- `make run` exports the React Email template first, then Go renders the generated briefing into `cache/briefing_email.html`.
- Email sending is intentionally not wired yet; the rendered HTML file is the handoff point until mail infrastructure is available.
