package exchange

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gorilla/websocket"
)

// TODO изменить json нотацию
// BookTickerMessage - структура для данных WebSocket о ценах (JSON)
type BookTickerMessage struct {
	Symbol   string `json:"Symbol"`
	BidPrice string `json:"BidPrice"`
	BidQty   string `json:"BidQty"`
	AskPrice string `json:"AskPrice"`
	AskQty   string `json:"AskQty"`
	Time     int64  `json:"Time"`
}

// TODO изменить json нотацию
// OrderMessage - структура для данных WebSocket об ордерах (JSON)
type OrderMessage struct {
	Symbol     string `json:"Symbol"`
	Id         string `json:"Id"`
	Price      string `json:"Price"`
	Quantity   string `json:"Quantity"`
	TradeId    string `json:"TradeId"`
	Status     int32  `json:"Status"`
	CreateTime int64  `json:"CreateTime"`
}

// OrderUpdate - структура для обновлений состояния ордеров
type OrderUpdate struct {
	OrderID   string
	Price     float64
	Quantity  float64
	Status    string
	Timestamp int64
}

const (
	reconnectInterval = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 5 * time.Second
	maxReconnects     = 3
)

//TODO: Добавить обработку ошибок, подумать о реконнектах, подумать о том, чтобы как закрывать соединение при ошибке
func (c *MEXCClient) SubscribePrice(ctx context.Context, priceChan chan<- float64) error {
	stream := fmt.Sprintf("spot@public.bookTicker.v3.api@%s", c.symbol)
	return c.subscribePublic(ctx, stream, func(message []byte) {
		log.Printf("Price Update: %s", string(message))
		return
		// Обработка сообщения и извлечение цены
		// var ticker BookTickerMessage
		// if err := json.Unmarshal(message, &ticker); err != nil {
		// 	log.Printf("Ошибка десериализации сообщения: %v", err)
		// 	return
		// }
		// price, err := strconv.ParseFloat(ticker.BidPrice, 64)
		// if err != nil {
		// 	log.Printf("Ошибка преобразования BidPrice: %v", err)
		// 	return
		// }
		// select {
		// case priceChan <- price:
		// case <-ctx.Done():
		// 	return
		// }
	})
}

// SubscribePublic - подписка на публичный WebSocket-канал с ограничением на количество реконнектов
func (c *MEXCClient) subscribePublic(ctx context.Context, stream string, handler func([]byte)) error {
	reconnectAttempts := 0

	for {
		err := c.connectAndSubscribe(ctx, stream, handler)
		if err != nil {
			log.Printf("Ошибка подписки: %v. Попытка повторного подключения %d из %d через %v...", err, reconnectAttempts+1, maxReconnects, reconnectInterval)
			reconnectAttempts++
			if reconnectAttempts >= maxReconnects {
				return fmt.Errorf("превышено максимальное количество попыток подключения: %v", err)
			}
			time.Sleep(reconnectInterval)
		} else {
			// Сбросить счетчик, если подключение успешно
			reconnectAttempts = 0
		}
	}
}

func (c *MEXCClient) connectAndSubscribe(ctx context.Context, stream string, handler func([]byte)) error {
	conn, _, err := websocket.DefaultDialer.Dial(c.wsURL, nil)
	if err != nil {
		return fmt.Errorf("ошибка подключения к WebSocket: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(readTimeout)); err != nil {
		log.Printf("Ошибка установки дедлайна чтения: %v", err)
		return fmt.Errorf("ошибка установки дедлайна чтения: %v", err)
	}
	
	if err := conn.SetWriteDeadline(time.Now().Add(writeTimeout)); err != nil {
		log.Printf("Ошибка установки дедлайна записи: %v", err)
		return fmt.Errorf("ошибка установки дедлайна записи: %v", err)
	}

	go func() {
		defer func() {
			if err := conn.Close(); err != nil {
				log.Printf("Ошибка закрытия WebSocket соединения: %v", err)
			}
		}()
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
	if err := conn.WriteJSON(msg); err != nil {
		return fmt.Errorf("ошибка отправки сообщения подписки: %v", err)
	}

	return nil
}

func (c *MEXCClient) SubscribeOrders(ctx context.Context, stream string, handler func([]byte)) error {
	return c.subscribePublic(ctx, stream, handler)
}

//TODO: проверить тк фунция сгерерирована
// SubscribePrivate - подписка на приватный WebSocket-канал
func (c *MEXCClient) SubscribePrivate(ctx context.Context, stream string, handler func([]byte)) error {
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
