package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/gorilla/websocket"
	"net/http"
	"scalpingbot/internal/logger"
	"sync"
	"time"
)

const MaxOpenOrders = 500

// Exchange - интерфейс для работы с биржей
type Exchange interface {
	GetPrice(ctx context.Context, symbol string) (float64, error)
	GetAccountInfo(ctx context.Context) (*AccountInfo, error)
	PlaceOrder(ctx context.Context, req SpotOrderRequest) (*OrderResponse, error)
	GetAllOrders(ctx context.Context, symbol string, startTime, endTime int64) ([]OrderInfo, error)
	GetOpenOrders(ctx context.Context, symbol string) ([]OrderInfo, error)
	GetKlines(ctx context.Context, symbol, interval string, limit int) ([]Kline, error)
	CancelOrder(ctx context.Context, symbol, orderID string) error
	SubscribeOrderUpdates(ctx context.Context, updateCh chan<- OrderUpdate) error
}

// MEXCClient - клиент для работы с MEXC API
type MEXCClient struct {
	client      *http.Client
	apiKey      string
	secretKey   string
	symbol      string
	baseURL     string
	wsURL       string
	conn        *websocket.Conn
	connMu      sync.RWMutex
	reconnectCh chan struct{}
	logger      logger.Logger
}

// NewMEXCClient - конструктор клиента
func NewMEXCClient(apiKey, secretKey, symbol string, logLogger logger.Logger) *MEXCClient {
	return &MEXCClient{
		client: &http.Client{
			Timeout: 3 * time.Second,
		},
		apiKey:      apiKey,
		secretKey:   secretKey,
		symbol:      symbol,
		baseURL:     "https://api.mexc.com",
		wsURL:       "wss://wbs-api.mexc.com/ws?listenKey=%s",
		reconnectCh: make(chan struct{}, 1),
		logger:      logLogger,
	}
}

// sign - генерация HMAC-SHA256 подписи
func (c *MEXCClient) sign(query string) string {
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(query))
	return hex.EncodeToString(mac.Sum(nil))
}
