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

const (
	Buy  = "BUY"
	Sell = "SELL"

	// типы ордеров
	Limit = "LIMIT"

	// статусы ордеров
	New             = "NEW"
	PartiallyFilled = "PARTIALLY_FILLED"
	Filled          = "FILLED"
)

// OrderInfo — структура одного ордера
type OrderInfo struct {
	Symbol      string `json:"symbol"`
	OrderID     string `json:"orderId"`
	Price       string `json:"price"`
	OrigQty     string `json:"origQty"`
	ExecutedQty string `json:"executedQty"`
	Status      string `json:"status"` // NEW, PARTIALLY_FILLED, FILLED, CANCELED, etc.
	Type        string `json:"type"`
	Side        string `json:"side"`
	Time        int64  `json:"time"`
	UpdateTime  int64  `json:"updateTime"`
}

// GetAllOrders — получить все ордера по символу
func (c *MEXCClient) GetAllOrders(ctx context.Context, symbol string, startTime, endTime int64) ([]OrderInfo, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("startTime", strconv.FormatInt(startTime, 10))
	q.Set("endTime", strconv.FormatInt(endTime, 10))
	q.Set("timestamp", timestamp)

	signature := c.sign(q.Encode())
	q.Set("signature", signature)

	url := fmt.Sprintf("%s/api/v3/allOrders?%s", c.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	var orders []OrderInfo
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("не удалось декодировать ответ: %w, тело: %s", err, string(body))
	}

	// спим 0.2 сек
	time.Sleep(200 * time.Millisecond)

	return orders, nil
}

func (c *MEXCClient) GetOpenOrders(ctx context.Context, symbol string) ([]OrderInfo, error) {
	timestamp := strconv.FormatInt(time.Now().UnixMilli(), 10)

	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("timestamp", timestamp)

	signature := c.sign(q.Encode())
	q.Set("signature", signature)

	url := fmt.Sprintf("%s/api/v3/openOrders?%s", c.baseURL, q.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	var orders []OrderInfo
	if err := json.Unmarshal(body, &orders); err != nil {
		return nil, fmt.Errorf("не удалось декодировать ответ: %w, тело: %s", err, string(body))
	}

	// Задержка для предотвращения превышения лимитов API
	time.Sleep(200 * time.Millisecond)

	return orders, nil
}

// SpotOrderRequest - структура для создания ордера через REST API
type SpotOrderRequest struct {
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"`
	Type      string  `json:"type"`
	Quantity  float64 `json:"quantity"`
	Price     float64 `json:"price,omitempty"`
	Timestamp int64   `json:"timestamp"`
}

// OrderResponse - ответ от API на создание ордера
type OrderResponse struct {
	Symbol       string `json:"symbol"`
	OrderID      string `json:"orderId"`
	OrderListID  int    `json:"orderListId"`
	Price        string `json:"price"`
	OrigQty      string `json:"origQty"`
	Type         string `json:"type"`
	Side         string `json:"side"`
	TransactTime int64  `json:"transactTime"`
}

// NewOrder - создание нового ордера через REST API
func (c *MEXCClient) PlaceOrder(ctx context.Context, req SpotOrderRequest) (*OrderResponse, error) {
	req.Timestamp = time.Now().UnixMilli()
	req.Symbol = c.symbol

	query := c.buildOrderQuery(req)
	signature := c.sign(query.Encode())
	query.Set("signature", signature)

	urlEndpoint := fmt.Sprintf("%s/api/v3/order?%s", c.baseURL, query.Encode())
	httpReq, err := http.NewRequest("POST", urlEndpoint, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("X-MEXC-APIKEY", c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка API: %s, тело: %s", resp.Status, string(body))
	}

	var orderResp OrderResponse
	if err := json.Unmarshal(body, &orderResp); err != nil {
		return nil, fmt.Errorf("не удалось декодировать ответ PlaceOrder: %w, тело: %s", err, string(body))
	}

	// спим 0.2 сек (чтобы не было ошибки апи too many requests)
	time.Sleep(200 * time.Millisecond)

	return &orderResp, nil
}

func (c *MEXCClient) CancelOrder(ctx context.Context, symbol, orderID string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", orderID)
	params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))

	signature := c.sign(params.Encode())
	params.Set("signature", signature)

	url := fmt.Sprintf("%s/api/v3/order?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("создание запроса отмены ордера: %w", err)
	}
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("запрос отмены ордера: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("ошибка API отмены ордера: %s, тело: %s", resp.Status, string(body))
	}

	// спим 0.2 сек (чтобы не было ошибки апи too many requests)
	time.Sleep(200 * time.Millisecond)

	return nil
}

func (c *MEXCClient) buildOrderQuery(req SpotOrderRequest) url.Values {
	q := url.Values{}
	q.Set("symbol", req.Symbol)
	q.Set("side", req.Side)
	q.Set("type", req.Type)
	q.Set("quantity", fmt.Sprintf("%.8f", req.Quantity))
	if req.Type == "LIMIT" {
		q.Set("price", fmt.Sprintf("%.8f", req.Price))
	}
	q.Set("timestamp", strconv.FormatInt(req.Timestamp, 10))
	return q
}
