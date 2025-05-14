package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	KlineInterval1m  = "1m"
	KlineInterval5m  = "5m"
	KlineInterval15m = "15m"
	KlineInterval30m = "30m"
	KlineInterval1h  = "1h"
)

type Kline struct {
	OpenTime  int64   `json:"-"`
	Open      float64 `json:"-"`
	High      float64 `json:"-"`
	Low       float64 `json:"-"`
	Close     float64 `json:"-"`
	Volume    float64 `json:"-"`
	CloseTime int64   `json:"-"`
}

func (c *MEXCClient) GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error) {
	url := fmt.Sprintf("%s/api/v3/klines?symbol=%s&interval=%s&limit=%d", c.baseURL, symbol, interval, limit+1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("api error: %s, body: %s", resp.Status, string(body))
	}

	var raw [][]any
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	now := time.Now().UnixMilli()
	klines := make([]Kline, 0, limit)

	for i, row := range raw {
		if len(row) < 7 {
			return nil, fmt.Errorf("kline #%d: not enough fields", i)
		}

		closeTime, err := toInt64(row[6])
		if err != nil || closeTime > now {
			//log.Printf("kline #%d: not finished, continue: %d", i, closeTime)
			continue // свеча ещё не завершена
		}

		openTime, err := toInt64(row[0])
		if err != nil {
			return nil, fmt.Errorf("parse openTime (row %d): %w", i, err)
		}
		open, err := parseStringFloat(row[1])
		if err != nil {
			return nil, fmt.Errorf("parse open (row %d): %w", i, err)
		}
		high, err := parseStringFloat(row[2])
		if err != nil {
			return nil, fmt.Errorf("parse high (row %d): %w", i, err)
		}
		low, err := parseStringFloat(row[3])
		if err != nil {
			return nil, fmt.Errorf("parse low (row %d): %w", i, err)
		}
		closeVal, err := parseStringFloat(row[4])
		if err != nil {
			return nil, fmt.Errorf("parse close (row %d): %w", i, err)
		}
		volume, err := parseStringFloat(row[5])
		if err != nil {
			return nil, fmt.Errorf("parse volume (row %d): %w", i, err)
		}

		klines = append(klines, Kline{
			OpenTime:  openTime,
			Open:      open,
			High:      high,
			Low:       low,
			Close:     closeVal,
			Volume:    volume,
			CloseTime: closeTime,
		})
	}

	if len(klines) > limit {
		klines = klines[len(klines)-limit:]
	}

	return klines, nil
}

func parseStringFloat(v any) (float64, error) {
	s, ok := v.(string)
	if !ok {
		return 0, fmt.Errorf("expected string, got %T", v)
	}
	return strconv.ParseFloat(s, 64)
}

func toInt64(v any) (int64, error) {
	switch t := v.(type) {
	case float64:
		return int64(t), nil
	default:
		return 0, fmt.Errorf("expected number, got %T", v)
	}
}
