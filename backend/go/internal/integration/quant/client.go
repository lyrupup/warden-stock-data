// Package quant 提供 Go 服务调用 Python quant 服务的内部 HTTP 客户端。
package quant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// IQuantClient 抽象 Python quant 内部 API，便于单测 mock。
type IQuantClient interface {
	Health(ctx context.Context) error
	Catalog(ctx context.Context) (*CatalogResponse, error)
	CollectSecurities(ctx context.Context, market string) (int, error)
	CollectCalendar(ctx context.Context, market, fromDate, toDate string) (int, error)
	CollectKline(ctx context.Context, req CollectKlineRequest) (*CollectKlineResponse, error)
	BatchIndicators(ctx context.Context, req IndicatorRequest) (*IndicatorResponse, error)
	SeriesIndicators(ctx context.Context, req IndicatorSeriesRequest) (*IndicatorSeriesResponse, error)
}

// Client 调用 Python quant 内部 API（/internal/v1/*）。
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // 采集单批可能较慢
		},
	}
}

type CollectKlineRequest struct {
	Codes    []string `json:"codes"`
	Mode     string   `json:"mode"` // full / incremental
	FromDate string   `json:"from_date,omitempty"`
	ToDate   string   `json:"to_date,omitempty"`
	Market   string   `json:"market"`
}

type CollectResult struct {
	Code            string `json:"code"`
	Status          string `json:"status"` // ok / skipped / failed
	Rows            int    `json:"rows"`
	LatestTradeDate string `json:"latest_trade_date,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

type CollectKlineResponse struct {
	Results []CollectResult `json:"results"`
}

type IndicatorRequest struct {
	Codes     []string `json:"codes"`
	Types     []string `json:"types"`
	Period    string   `json:"period"`
	Adjust    string   `json:"adjust"`
	TradeDate string   `json:"trade_date,omitempty"`
	Market    string   `json:"market"`
}

type IndicatorResult struct {
	StockCode string            `json:"stock_code"`
	TradeDate string            `json:"trade_date,omitempty"`
	Values    map[string]string `json:"values"`
}

type IndicatorResponse struct {
	Results []IndicatorResult `json:"results"`
}

type IndicatorSeriesRequest struct {
	Code     string   `json:"code"`
	Types    []string `json:"types"`
	Period   string   `json:"period"`
	Adjust   string   `json:"adjust"`
	Limit    int      `json:"limit"`
	Offset   int      `json:"offset"`
	FromDate string   `json:"from_date,omitempty"`
	ToDate   string   `json:"to_date,omitempty"`
	Market   string   `json:"market"`
}

type IndicatorSeriesResponse struct {
	StockCode  string            `json:"stock_code"`
	Indicators []IndicatorResult `json:"indicators"`
}

type CatalogItem struct {
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Group       string                 `json:"group"`
	ValueType   string                 `json:"value_type"`
	Params      map[string]interface{} `json:"params"`
	Implemented bool                   `json:"implemented"`
}

type CatalogResponse struct {
	Indicators   []CatalogItem `json:"indicators"`
	DefaultTypes []string      `json:"default_types"`
}

func (c *Client) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("quant health: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) Catalog(ctx context.Context) (*CatalogResponse, error) {
	var out CatalogResponse
	if err := c.doJSON(ctx, http.MethodGet, "/internal/v1/catalog", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) CollectSecurities(ctx context.Context, market string) (int, error) {
	var out struct {
		Count int `json:"count"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/internal/v1/collect/securities", map[string]string{"market": market}, &out); err != nil {
		return 0, err
	}
	return out.Count, nil
}

func (c *Client) CollectCalendar(ctx context.Context, market, fromDate, toDate string) (int, error) {
	var out struct {
		Count int `json:"count"`
	}
	body := map[string]string{"market": market}
	if fromDate != "" {
		body["from_date"] = fromDate
	}
	if toDate != "" {
		body["to_date"] = toDate
	}
	if err := c.doJSON(ctx, http.MethodPost, "/internal/v1/collect/calendar", body, &out); err != nil {
		return 0, err
	}
	return out.Count, nil
}

func (c *Client) CollectKline(ctx context.Context, req CollectKlineRequest) (*CollectKlineResponse, error) {
	var out CollectKlineResponse
	if err := c.doJSON(ctx, http.MethodPost, "/internal/v1/collect/kline", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) BatchIndicators(ctx context.Context, req IndicatorRequest) (*IndicatorResponse, error) {
	var out IndicatorResponse
	if err := c.doJSON(ctx, http.MethodPost, "/internal/v1/indicators", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) SeriesIndicators(ctx context.Context, req IndicatorSeriesRequest) (*IndicatorSeriesResponse, error) {
	var out IndicatorSeriesResponse
	if err := c.doJSON(ctx, http.MethodPost, "/internal/v1/indicators/series", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, r)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("quant %s %s: status %d body %s", method, path, resp.StatusCode, string(data))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return err
		}
	}
	return nil
}
