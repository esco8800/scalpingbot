package exchange

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

// MEXCClient - клиент для работы с MEXC API
type MEXCClient struct {
	apiKey    string
	secretKey string
	baseURL   string
	wsURL     string
}

// NewMEXCClient - конструктор клиента
func NewMEXCClient(apiKey, secretKey string) *MEXCClient {
	return &MEXCClient{
		apiKey:    apiKey,
		secretKey: secretKey,
		baseURL:   "https://api.mexc.com",
		wsURL:     "wss://wbs.mexc.com/ws",
	}
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
	OrderId string `json:"orderId"`
}

// NewOrder - создание нового ордера через REST API
func (c *MEXCClient) NewOrder(req SpotOrderRequest) (*OrderResponse, error) {
	req.Timestamp = time.Now().UnixMilli()
	query := url.Values{}
	query.Set("symbol", req.Symbol)
	query.Set("side", req.Side)
	query.Set("type", req.Type)
	query.Set("quantity", fmt.Sprintf("%.8f", req.Quantity))
	if req.Type == "LIMIT" {
		query.Set("price", fmt.Sprintf("%.8f", req.Price))
	}
	query.Set("timestamp", strconv.FormatInt(req.Timestamp, 10))

	signature := c.sign(query.Encode())
	query.Set("signature", signature)

	url := fmt.Sprintf("%s/api/v3/order?%s", c.baseURL, query.Encode())
	httpReq, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("X-MEXC-APIKEY", c.apiKey)

	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ошибка API: %s", resp.Status)
	}

	var orderResp OrderResponse
	if err := json.NewDecoder(resp.Body).Decode(&orderResp); err != nil {
		return nil, err
	}
	return &orderResp, nil
}

// sign - генерация HMAC-SHA256 подписи
func (c *MEXCClient) sign(query string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}

// BookTickerMessage - структура для данных WebSocket о ценах (JSON)
type BookTickerMessage struct {
	Symbol   string `json:"s"`
	BidPrice string `json:"b"`
	BidQty   string `json:"B"`
	AskPrice string `json:"a"`
	AskQty   string `json:"A"`
	Time     int64  `json:"t"`
}

// OrderMessage - структура для данных WebSocket об ордерах (JSON)
type OrderMessage struct {
	Symbol     string `json:"symbol"`
	Id         string `json:"id"`
	Price      string `json:"price"`
	Quantity   string `json:"quantity"`
	TradeId    string `json:"tradeId"`
	Status     int32  `json:"status"`
	CreateTime int64  `json:"createTime"`
}

// SubscribePublic - подписка на публичный WebSocket-канал
func (c *MEXCClient) SubscribePublic(stream string, handler func([]byte)) error {
	conn, _, err := websocket.DefaultDialer.Dial(c.wsURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка подключения к WebSocket: %v", err)
	}

	go func() {
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Ошибка чтения WebSocket: %v", err)
				return
			}
			handler(message)
		}
	}()

	msg := map[string]interface{}{
		"method": "SUBSCRIPTION",
		"params": []string{stream},
	}
	return conn.WriteJSON(msg)
}

// SubscribePrivate - подписка на приватный WebSocket-канал
func (c *MEXCClient) SubscribePrivate(stream string, handler func([]byte)) error {
	conn, _, err := websocket.DefaultDialer.Dial(c.wsURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка подключения к WebSocket: %v", err)
	}

	timestamp := time.Now().UnixMilli()
	query := fmt.Sprintf("apiKey=%s&reqTime=%d", c.apiKey, timestamp)
	signature := c.sign(query)
	authMsg := map[string]interface{}{
		"method": "LOGIN",
		"params": map[string]string{
			"apiKey":    c.apiKey,
			"reqTime":   strconv.FormatInt(timestamp, 10),
			"signature": signature,
		},
	}
	if err := conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return fmt.Errorf("ошибка аутентификации WebSocket: %v", err)
	}

	go func() {
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				log.Printf("Ошибка чтения WebSocket: %v", err)
				return
			}
			handler(message)
		}
	}()

	msg := map[string]interface{}{
		"method": "SUBSCRIPTION",
		"params": []string{stream},
	}
	return conn.WriteJSON(msg)
}
