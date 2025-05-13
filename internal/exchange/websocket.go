package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"log"
	"time"
)

// Таймеры для периодического обновления соединения и пинга
const reconnectInterval = 30 * time.Minute
const pingInterval = 30 * time.Second
const maxReconnectAttempts = 5

// OrderUpdate - структура для обновлений ордеров
type OrderUpdate struct {
	Symbol    string `json:"s"`
	OrderId   string `json:"i"`
	Status    string `json:"S"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	Timestamp int64  `json:"T"`
}

// SubscribeOrderUpdates - подписка на обновления ордеров через WebSocket
func (c *MEXCClient) SubscribeOrderUpdates(ctx context.Context, updateCh chan<- OrderUpdate) error {
	// Функция для создания нового WebSocket соединения
	connect := func() error {
		c.connMu.Lock()
		defer c.connMu.Unlock()

		// Формируем параметры подписки
		timestamp := time.Now().UnixMilli()
		query := fmt.Sprintf("apiKey=%s&reqTime=%d", c.apiKey, timestamp)
		signature := c.sign(query)

		// Устанавливаем соединение
		url := fmt.Sprintf("%s?apiKey=%s&reqTime=%d&signature=%s", c.wsURL, c.apiKey, timestamp, signature)
		conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, nil)
		if err != nil {
			log.Printf("Ошибка подключения к WebSocket: %v", err)
			return fmt.Errorf("failed to connect to WebSocket: %w", err)
		}

		// Устанавливаем таймауты
		conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

		// Подписываемся на обновления ордеров
		subscribeMsg := map[string]interface{}{
			"method": "SUBSCRIPTION",
			"params": []string{"spot@private.orders.v3.api"},
		}
		if err := conn.WriteJSON(subscribeMsg); err != nil {
			conn.Close()
			return fmt.Errorf("failed to subscribe: %w", err)
		}

		log.Printf("Успещно подписались на обновления ордеров")
		c.conn = conn
		return nil
	}

	// Запускаем горутину для обработки сообщений
	go func() {
		reconnectTimer := time.NewTimer(reconnectInterval)
		pingTicker := time.NewTicker(pingInterval)
		defer func() {
			reconnectTimer.Stop()
			pingTicker.Stop()
			c.connMu.Lock()
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
			}
			c.connMu.Unlock()
		}()

		// Первоначальное подключение
		for attempt := 0; attempt < maxReconnectAttempts; attempt++ {
			if err := connect(); err == nil {
				log.Printf("Первоначальное вебсокет подключение успешно")
				break
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second * time.Duration(attempt+1)):
				continue
			}
		}

		for {
			select {
			case <-ctx.Done():
				return

			case <-c.reconnectCh:
				// Принудительное переподключение
				c.connMu.Lock()
				if c.conn != nil {
					c.conn.Close()
					c.conn = nil
				}
				c.connMu.Unlock()

				for attempt := 0; attempt < maxReconnectAttempts; attempt++ {
					if err := connect(); err == nil {
						log.Printf("Реконнект к вебсокету успешно")
						break
					}
					select {
					case <-ctx.Done():
						return
					case <-time.After(time.Second * time.Duration(attempt+1)):
						continue
					}
				}

			case <-reconnectTimer.C:
				// Периодическое переподключение
				select {
				case c.reconnectCh <- struct{}{}:
					// Сигнал отправлен
				default:
					log.Printf("reconnectCh full, skipping signal")
				}
				log.Printf("Периодическое переподключение к вебсокету")
				reconnectTimer.Reset(reconnectInterval)

			case <-pingTicker.C:
				// Отправка пинга для поддержания соединения
				c.connMu.RLock()
				if c.conn != nil {
					if err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
						select {
						case c.reconnectCh <- struct{}{}:
							// Сигнал отправлен
						default:
							log.Printf("reconnectCh full, skipping signal")
						}
					}
					log.Printf("Отправлен пинг на вебсокет ордеров соединение")
				}
				c.connMu.RUnlock()

			default:
				// Чтение сообщений
				c.connMu.RLock()
				if c.conn == nil {
					c.connMu.RUnlock()
					log.Printf("Вебсокет соединение закрыто, пропускаем чтение")
					continue
				}

				var msg map[string]interface{}
				c.conn.SetReadDeadline(time.Now().Add(time.Minute))
				err := c.conn.ReadJSON(&msg)
				c.connMu.RUnlock()

				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						log.Printf("Читаем из закрытого соедининия: %v", err)
						select {
						case c.reconnectCh <- struct{}{}:
							// Сигнал отправлен
						default:
							log.Printf("reconnectCh full, skipping signal")
						}
						continue
					}
					log.Printf("Ошибка чтения сообщения: msg %v err %v", msg, err)
					select {
					case c.reconnectCh <- struct{}{}:
						// Сигнал отправлен
					default:
						log.Printf("reconnectCh full, skipping signal")
					}
					continue
				}

				// Обработка обновлений ордеров
				if data, ok := msg["d"].(map[string]interface{}); ok {
					var update OrderUpdate
					dataBytes, _ := json.Marshal(data)
					if err := json.Unmarshal(dataBytes, &update); err == nil {
						select {
						case updateCh <- update:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}
	}()

	return nil
}
