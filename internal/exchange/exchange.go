package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"sync"
)

// Exchange - интерфейс для работы с биржей
type Exchange interface {
	GetCurrentPrice(ctx context.Context) (float64, error)
	PlaceBuyOrder(ctx context.Context, price, amount float64) (string, error)
	PlaceSellOrder(ctx context.Context, price, amount float64) (string, error)
	SubscribePrice(ctx context.Context, priceChan chan<- float64)
	SubscribeOrders(ctx context.Context, orderChan chan<- OrderUpdate)
}

// OrderUpdate - структура для обновлений состояния ордеров
type OrderUpdate struct {
	OrderID   string
	Price     float64
	Quantity  float64
	Status    string
	Timestamp int64
}

// MEXCExchange - реализация интерфейса для MEXC
type MEXCExchange struct {
	client       *MEXCClient // Используем MEXCClient из client.go
	symbol       string
	currentPrice float64
	mu           sync.Mutex
}

// NewMEXCExchange - создание нового клиента MEXC
func NewMEXCExchange(apiKey, secretKey, symbol string) *MEXCExchange {
	client := NewMEXCClient(apiKey, secretKey) // Функция из client.go
	return &MEXCExchange{
		client: client,
		symbol: symbol,
	}
}

// GetCurrentPrice - возвращает последнюю известную цену из WebSocket
func (e *MEXCExchange) GetCurrentPrice(ctx context.Context) (float64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.currentPrice == 0 {
		return 0, fmt.Errorf("цена ещё не получена через WebSocket")
	}
	return e.currentPrice, nil
}

// PlaceBuyOrder - выставление рыночного ордера на покупку через REST API
func (e *MEXCExchange) PlaceBuyOrder(ctx context.Context, price, amount float64) (string, error) {
	quantity := amount / price
	orderReq := SpotOrderRequest{ // Структура из client.go
		Symbol:   e.symbol,
		Side:     "BUY",
		Type:     "MARKET",
		Quantity: quantity,
	}
	resp, err := e.client.NewOrder(orderReq)
	if err != nil {
		return "", fmt.Errorf("ошибка покупки: %v", err)
	}
	return resp.OrderId, nil
}

// PlaceSellOrder - выставление лимитного ордера на продажу через REST API
func (e *MEXCExchange) PlaceSellOrder(ctx context.Context, price, amount float64) (string, error) {
	quantity := amount / price
	orderReq := SpotOrderRequest{ // Структура из client.go
		Symbol:   e.symbol,
		Side:     "SELL",
		Type:     "LIMIT",
		Price:    price,
		Quantity: quantity,
	}
	resp, err := e.client.NewOrder(orderReq)
	if err != nil {
		return "", fmt.Errorf("ошибка продажи: %v", err)
	}
	return resp.OrderId, nil
}

// SubscribePrice - подписка на обновления цены через WebSocket
func (e *MEXCExchange) SubscribePrice(ctx context.Context, priceChan chan<- float64) {
	stream := fmt.Sprintf("spot@public.bookTicker.v3.api@%s", e.symbol)
	err := e.client.SubscribePublic(stream, func(data []byte) {
		var ticker BookTickerMessage // Структура из client.go
		if err := json.Unmarshal(data, &ticker); err != nil {
			log.Printf("Ошибка десериализации BookTicker JSON: %v", err)
			return
		}
		bid, err := strconv.ParseFloat(ticker.BidPrice, 64)
		if err != nil {
			log.Printf("Ошибка преобразования BidPrice: %v", err)
			return
		}
		ask, err := strconv.ParseFloat(ticker.AskPrice, 64)
		if err != nil {
			log.Printf("Ошибка преобразования AskPrice: %v", err)
			return
		}
		price := (bid + ask) / 2
		e.mu.Lock()
		e.currentPrice = price
		e.mu.Unlock()
		select {
		case priceChan <- price:
		case <-ctx.Done():
			return
		}
	})
	if err != nil {
		log.Printf("Ошибка подписки на цену: %v", err)
	}
}

// statusToString - преобразование числового статуса в строку
func statusToString(status int32) string {
	switch status {
	case 1:
		return "NEW"
	case 2:
		return "FILLED"
	case 3:
		return "PARTIALLY_FILLED"
	case 4:
		return "CANCELED"
	case 5:
		return "PARTIALLY_CANCELED"
	default:
		return fmt.Sprintf("UNKNOWN_%d", status)
	}
}

// SubscribeOrders - подписка на обновления ордеров через WebSocket
func (e *MEXCExchange) SubscribeOrders(ctx context.Context, orderChan chan<- OrderUpdate) {
	stream := "spot@private.orders.v3.api"
	err := e.client.SubscribePrivate(stream, func(data []byte) {
		var order OrderMessage // Структура из client.go
		if err := json.Unmarshal(data, &order); err != nil {
			log.Printf("Ошибка десериализации Order JSON: %v", err)
			return
		}
		price, err := strconv.ParseFloat(order.Price, 64)
		if err != nil {
			log.Printf("Ошибка преобразования Price: %v", err)
			return
		}
		quantity, err := strconv.ParseFloat(order.Quantity, 64)
		if err != nil {
			log.Printf("Ошибка преобразования Quantity: %v", err)
			return
		}
		update := OrderUpdate{
			OrderID:   order.Id,
			Price:     price,
			Quantity:  quantity,
			Status:    statusToString(order.Status),
			Timestamp: order.CreateTime,
		}
		select {
		case orderChan <- update:
		case <-ctx.Done():
			return
		}
	})
	if err != nil {
		log.Printf("Ошибка подписки на ордера: %v", err)
	}
}
