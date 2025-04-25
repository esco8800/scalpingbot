package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
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
)

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

	url := fmt.Sprintf("%s/api/v3/order?%s", c.baseURL, query.Encode())
	httpReq, err := http.NewRequest("POST", url, nil)
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
		return nil, fmt.Errorf("не удалось декодировать ответ: %w, тело: %s", err, string(body))
	}
	return &orderResp, nil
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

// sign - генерация HMAC-SHA256 подписи
func (c *MEXCClient) sign(query string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}
