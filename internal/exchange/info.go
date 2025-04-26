package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// BalanceInfo — структура баланса по валюте
type BalanceInfo struct {
	Asset  string `json:"asset"`
	Free   string `json:"free"`
	Locked string `json:"locked"`
}

// AccountInfo — общий ответ от /api/v3/account
type AccountInfo struct {
	Balances []BalanceInfo `json:"balances"`
}

// GetUsdtBalance возвращает свободный баланс USDT
func (a *AccountInfo) GetUsdtBalance() (float64, error) {
	for _, b := range a.Balances {
		if b.Asset == "USDT" {
			free, err := strconv.ParseFloat(b.Free, 64)
			return free, err
		}
	}
	return 0, fmt.Errorf("баланс USDT не найден")
}

// GetKasBalance возвращает свободный баланс KAS
func (a *AccountInfo) GetKasBalance() (float64, error) {
	for _, b := range a.Balances {
		if b.Asset == "KAS" {
			free, err := strconv.ParseFloat(b.Free, 64)
			return free, err
		}
	}
	return 0, fmt.Errorf("баланс KAS не найден")
}

// GetAccountInfo — получает информацию о всех балансах аккаунта
func (c *MEXCClient) GetAccountInfo(ctx context.Context) (*AccountInfo, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	q := url.Values{}
	q.Set("timestamp", timestamp)

	signature := c.sign(q.Encode())
	q.Set("signature", signature)

	urlEndpoint := fmt.Sprintf("%s/api/v3/account?%s", c.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", urlEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка API: %s, тело: %s", resp.Status, string(body))
	}

	var accountInfo AccountInfo
	if err := json.Unmarshal(body, &accountInfo); err != nil {
		return nil, fmt.Errorf("не удалось декодировать ответ GetAccountInfo: %w, тело: %s", err, string(body))
	}

	return &accountInfo, nil
}

// TickerPrice — структура для парсинга ответа с ценой
type TickerPrice struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

// GetPrice — получить текущую цену любого символа, например "KASUSDT"
func (c *MEXCClient) GetPrice(ctx context.Context, symbol string) (float64, error) {
	urlEndpoint := fmt.Sprintf("%s/api/v3/ticker/price?symbol=%s", c.baseURL, symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", urlEndpoint, nil)
	if err != nil {
		return 0, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("ошибка API: %s, тело: %s", resp.Status, string(body))
	}

	var ticker TickerPrice
	if err := json.Unmarshal(body, &ticker); err != nil {
		return 0, fmt.Errorf("не удалось декодировать ответ: %w, тело: %s", err, string(body))
	}

	price, err := strconv.ParseFloat(ticker.Price, 64)
	if err != nil {
		return 0, fmt.Errorf("не удалось преобразовать цену: %w", err)
	}

	return price, nil
}
