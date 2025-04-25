package exchange

import (
	"context"
	"net/http"
	"time"
)

// Exchange - интерфейс для работы с биржей
type Exchange interface {
	PlaceOrder(ctx context.Context, price, amount float64) (string, error)
	SubscribePrice(ctx context.Context, priceChan chan<- float64)
	SubscribeOrders(ctx context.Context, orderChan chan<- OrderUpdate)
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
