package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"
)

// Exchange - интерфейс для работы с биржей
type Exchange interface {
	GetPrice(ctx context.Context, symbol string) (float64, error)
	GetAccountInfo(ctx context.Context) (*AccountInfo, error)
	PlaceOrder(ctx context.Context, req SpotOrderRequest) (*OrderResponse, error)
	GetAllOrders(ctx context.Context, symbol string) ([]OrderInfo, error)
	GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error)
	CancelOrder(ctx context.Context, symbol, orderID string) error
	//SubscribePrice(ctx context.Context, priceChan chan<- float64)
	//SubscribeOrders(ctx context.Context, orderChan chan<- OrderUpdate)
}

// MEXCClient - клиент для работы с MEXC API
type MEXCClient struct {
	client    *http.Client
	apiKey    string
	secretKey string
	symbol    string
	baseURL   string
	wsURL     string
}

// NewMEXCClient - конструктор клиента
func NewMEXCClient(apiKey, secretKey, symbol string) *MEXCClient {
	return &MEXCClient{
		client: &http.Client{
			Timeout: 2 * time.Second,
		},
		apiKey:    apiKey,
		secretKey: secretKey,
		symbol:    symbol,
		baseURL:   "https://api.mexc.com",
		wsURL:     "wss://wbs.mexc.com/ws",
	}
}

// sign - генерация HMAC-SHA256 подписи
func (c *MEXCClient) sign(query string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}
