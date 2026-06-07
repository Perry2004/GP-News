package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const (
	defaultYahooChartBaseURL  = "https://query1.finance.yahoo.com/v8/finance/chart/"
	dataSource                = "yahoo_chart_api"
	yahooMarketFetchTimeout   = 30 * time.Second
	yahooMarketRequestTimeout = 20 * time.Second
)

type Instrument struct {
	ID       string
	Name     string
	Category string
	Symbol   string
}

func yahooFinanceInstruments() []Instrument {
	return []Instrument{
		{ID: "nasdaq_composite", Name: "NASDAQ Composite", Category: "equity_index", Symbol: "^IXIC"},
		{ID: "sp500", Name: "S&P 500", Category: "equity_index", Symbol: "^GSPC"},
		{ID: "dow_jones_industrial_average", Name: "Dow Jones Industrial Average", Category: "equity_index", Symbol: "^DJI"},
		{ID: "russell_2000", Name: "Russell 2000", Category: "equity_index", Symbol: "^RUT"},
		{ID: "dax", Name: "DAX", Category: "equity_index", Symbol: "^GDAXI"},
		{ID: "ftse_100", Name: "FTSE 100", Category: "equity_index", Symbol: "^FTSE"},
		{ID: "cac_40", Name: "CAC 40", Category: "equity_index", Symbol: "^FCHI"},
		{ID: "euro_stoxx_50", Name: "Euro Stoxx 50", Category: "equity_index", Symbol: "^STOXX50E"},
		{ID: "nikkei_225", Name: "Nikkei 225", Category: "equity_index", Symbol: "^N225"},
		{ID: "hang_seng", Name: "Hang Seng Index", Category: "equity_index", Symbol: "^HSI"},
		{ID: "csi_300", Name: "CSI 300", Category: "equity_index", Symbol: "000300.SS"},
		{ID: "shanghai_composite", Name: "Shanghai Composite", Category: "equity_index", Symbol: "000001.SS"},
		{ID: "kospi", Name: "KOSPI", Category: "equity_index", Symbol: "^KS11"},
		{ID: "asx_200", Name: "ASX 200", Category: "equity_index", Symbol: "^AXJO"},
		{ID: "usd_jpy", Name: "USD/JPY", Category: "fx", Symbol: "JPY=X"},
		{ID: "usd_cny", Name: "USD/CNY", Category: "fx", Symbol: "CNY=X"},
		{ID: "usd_cnh", Name: "USD/CNH", Category: "fx", Symbol: "CNH=X"},
		{ID: "eur_usd", Name: "EUR/USD", Category: "fx", Symbol: "EURUSD=X"},
		{ID: "gbp_usd", Name: "GBP/USD", Category: "fx", Symbol: "GBPUSD=X"},
		{ID: "eur_jpy", Name: "EUR/JPY", Category: "fx", Symbol: "EURJPY=X"},
		{ID: "aud_usd", Name: "AUD/USD", Category: "fx", Symbol: "AUDUSD=X"},
		{ID: "dxy", Name: "DXY", Category: "fx", Symbol: "DX-Y.NYB"},
		{ID: "us_10y_treasury_yield", Name: "U.S. 10Y Treasury yield", Category: "rates", Symbol: "^TNX"},
		{ID: "brent_crude", Name: "Brent crude", Category: "commodity", Symbol: "BZ=F"},
		{ID: "wti_crude", Name: "WTI crude", Category: "commodity", Symbol: "CL=F"},
		{ID: "gold_futures", Name: "Gold futures", Category: "commodity", Symbol: "GC=F"},
		{ID: "copper_futures", Name: "Copper futures", Category: "commodity", Symbol: "HG=F"},
		{ID: "bitcoin", Name: "Bitcoin", Category: "crypto", Symbol: "BTC-USD"},
		{ID: "ethereum", Name: "Ethereum", Category: "crypto", Symbol: "ETH-USD"},
		{ID: "vix", Name: "VIX", Category: "risk", Symbol: "^VIX"},
		{ID: "skew", Name: "SKEW", Category: "risk", Symbol: "^SKEW"},
	}
}

type MarketValue struct {
	ID                 string
	Name               string
	Category           string
	Symbol             string
	Value              float64
	DailyChange        float64
	DailyChangePercent float64
	DailyChangeValid   bool
	Timestamp          time.Time
	History            []MarketHistoryPoint
	Source             string
}

type MarketHistoryPoint struct {
	Timestamp time.Time
	Close     float64
}

type FetchFailure struct {
	ID     string
	Symbol string
	Error  string
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Timestamp []int64 `json:"timestamp"`
			Meta      struct {
				Symbol             string   `json:"symbol"`
				Currency           *string  `json:"currency"`
				ExchangeName       string   `json:"exchangeName"`
				InstrumentType     string   `json:"instrumentType"`
				RegularMarketTime  *int64   `json:"regularMarketTime"`
				RegularMarketPrice *float64 `json:"regularMarketPrice"`
				PreviousClose      *float64 `json:"previousClose"`
				ChartPreviousClose *float64 `json:"chartPreviousClose"`
				DataGranularity    string   `json:"dataGranularity"`
				Range              string   `json:"range"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Close []*float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

func FetchYahooMarketValues(ctx context.Context, instruments []Instrument) ([]MarketValue, []FetchFailure) {
	return fetchYahooMarketValues(ctx, instruments, NewHTTPClient(), defaultYahooChartBaseURL)
}

func fetchYahooMarketValues(ctx context.Context, instruments []Instrument, client *http.Client, baseURL string) ([]MarketValue, []FetchFailure) {
	fetchCtx, cancel := context.WithTimeout(ctx, yahooMarketFetchTimeout)
	defer cancel()

	marketValuesChan := make(chan MarketValue, len(instruments))
	fetchFailuresChan := make(chan FetchFailure, len(instruments))

	for _, instrument := range instruments {
		go func(instrument Instrument) {
			reqCtx, cancel := context.WithTimeout(fetchCtx, yahooMarketRequestTimeout)
			defer cancel()
			value, err := fetchYahooMarketValue(reqCtx, client, baseURL, instrument)
			if err != nil {
				fetchFailuresChan <- FetchFailure{
					ID:     instrument.ID,
					Symbol: instrument.Symbol,
					Error:  err.Error(),
				}
				slog.Warn("Yahoo Finance fetch failed", "instrument", instrument.ID, "symbol", instrument.Symbol, "error", err)
				return
			}

			marketValuesChan <- value
			slog.Debug("Yahoo Finance data fetched",
				"instrument", value.ID,
				"symbol", value.Symbol,
				"value", value.Value,
				"timestamp", value.Timestamp.Format(time.RFC3339),
			)
		}(instrument) // Pass instrument as argument to avoid closure
	}

	marketValues := make([]MarketValue, 0, len(instruments))
	fetchFailures := make([]FetchFailure, 0)
	reported := make(map[string]bool, len(instruments))
	for len(reported) < len(instruments) {
		select {
		case value := <-marketValuesChan:
			marketValues = append(marketValues, value)
			reported[value.ID] = true
		case failure := <-fetchFailuresChan:
			fetchFailures = append(fetchFailures, failure)
			reported[failure.ID] = true
		case <-fetchCtx.Done():
			for _, instrument := range instruments {
				if !reported[instrument.ID] {
					fetchFailures = append(fetchFailures, FetchFailure{
						ID:     instrument.ID,
						Symbol: instrument.Symbol,
						Error:  fetchCtx.Err().Error(),
					})
					reported[instrument.ID] = true
					slog.Warn("Yahoo Finance fetch timed out", "instrument", instrument.ID, "symbol", instrument.Symbol, "error", fetchCtx.Err())
				}
			}
		}
	}
	slog.Debug("Yahoo Finance fetch completed", "success_count", len(marketValues), "failure_count", len(fetchFailures))
	return marketValues, fetchFailures
}

func fetchYahooMarketValue(ctx context.Context, client *http.Client, baseURL string, instrument Instrument) (MarketValue, error) {
	requestURL := strings.TrimRight(baseURL, "/") + "/" + url.PathEscape(instrument.Symbol) + "?range=5d&interval=1d"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return MarketValue{}, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	requestDump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		slog.Warn("Failed to dump Yahoo Finance request", "error", err)
	}
	slog.Debug("Fetching Yahoo Finance market value", "instrument", instrument.ID, "request", string(requestDump))

	resp, err := client.Do(req)
	if err != nil {
		return MarketValue{}, fmt.Errorf("request failed for %s: %w", instrument.ID, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // Limit to 1MB
	if err != nil {
		return MarketValue{}, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return MarketValue{}, fmt.Errorf("unexpected HTTP status %d: %s", resp.StatusCode, body)
	}

	value, err := parseYahooChartResponse(instrument, body)
	if err != nil {
		return MarketValue{}, err
	}

	return value, nil
}

func parseYahooChartResponse(instrument Instrument, body []byte) (MarketValue, error) {
	var response yahooChartResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return MarketValue{}, fmt.Errorf("decode Yahoo chart response: %w: %s", err, body)
	}

	if response.Chart.Error != nil {
		// Yahoo returned API error
		if response.Chart.Error.Description != "" {
			return MarketValue{}, fmt.Errorf("yahoo chart API error %s: %s", response.Chart.Error.Code, response.Chart.Error.Description)
		}
		return MarketValue{}, fmt.Errorf("yahoo chart API error %s", response.Chart.Error.Code)
	}

	if len(response.Chart.Result) == 0 {
		return MarketValue{}, fmt.Errorf("yahoo chart response has no result")
	}

	result := response.Chart.Result[0]
	value, timestamp, err := extractYahooValue(result.Meta.RegularMarketPrice, result.Meta.RegularMarketTime)
	if err != nil {
		return MarketValue{}, err
	}
	history := extractYahooHistory(result.Timestamp, result.Indicators.Quote)
	dailyChange, dailyChangePercent, dailyChangeValid := calculateDailyChange(value, result.Indicators.Quote)

	return MarketValue{
		ID:                 instrument.ID,
		Name:               instrument.Name,
		Category:           instrument.Category,
		Symbol:             instrument.Symbol,
		Value:              value,
		DailyChange:        dailyChange,
		DailyChangePercent: dailyChangePercent,
		DailyChangeValid:   dailyChangeValid,
		Timestamp:          timestamp,
		History:            history,
		Source:             dataSource,
	}, nil
}

func extractYahooValue(regularMarketPrice *float64, regularMarketTime *int64) (float64, time.Time, error) {
	if regularMarketPrice == nil {
		return 0, time.Time{}, fmt.Errorf("yahoo chart response has no regular market price")
	}
	if regularMarketTime == nil {
		return 0, time.Time{}, fmt.Errorf("yahoo chart response has no regular market time")
	}
	return *regularMarketPrice, time.Unix(*regularMarketTime, 0), nil
}

func extractYahooHistory(timestamps []int64, quotes []struct {
	Close []*float64 `json:"close"`
}) []MarketHistoryPoint {
	if len(timestamps) == 0 || len(quotes) == 0 {
		return nil
	}

	closes := quotes[0].Close
	limit := len(timestamps)
	if len(closes) < limit {
		limit = len(closes)
	}

	history := make([]MarketHistoryPoint, 0, limit)
	for i := 0; i < limit; i++ {
		if closes[i] == nil {
			continue
		}
		history = append(history, MarketHistoryPoint{
			Timestamp: time.Unix(timestamps[i], 0),
			Close:     *closes[i],
		})
	}
	return history
}

func calculateDailyChange(currentValue float64, quotes []struct {
	Close []*float64 `json:"close"`
}) (float64, float64, bool) {
	previousClose, ok := previousNonNullCloseBeforeLatestPoint(quotes)
	if !ok || previousClose == 0 {
		return 0, 0, false
	}

	dailyChange := currentValue - previousClose
	dailyChangePercent := dailyChange / previousClose * 100
	return dailyChange, dailyChangePercent, true
}

func previousNonNullCloseBeforeLatestPoint(quotes []struct {
	Close []*float64 `json:"close"`
}) (float64, bool) {
	if len(quotes) == 0 || len(quotes[0].Close) < 2 {
		return 0, false
	}

	closes := quotes[0].Close
	for i := len(closes) - 2; i >= 0; i-- {
		if closes[i] != nil {
			return *closes[i], true
		}
	}
	return 0, false
}
