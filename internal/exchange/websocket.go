package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"io"
	"log"
	"net/http"
	"time"
)

// Таймеры для периодического обновления соединения и пинга
const reconnectInterval = 15 * time.Minute
const pingInterval = 15 * time.Second
const maxReconnectAttempts = 5

// OrderUpdate - структура для обновлений ордеров
type OrderUpdate struct {
	OrderId   string  `json:"i"`
	Price     float64 `json:"p"`
	Timestamp int64   `json:"T"`
}

// SubscribeOrderUpdates - подписка на обновления ордеров через WebSocket
func (c *MEXCClient) SubscribeOrderUpdates(ctx context.Context, updateCh chan<- OrderUpdate) error {
	// Горутина для пинга
	c.ping(ctx)

	// Запускаем горутину для обработки сообщений
	go func() {
		reconnectTimer := time.NewTimer(reconnectInterval)
		defer func() {
			reconnectTimer.Stop()
			c.connMu.Lock()
			if c.conn != nil {
				c.conn.Close()
				c.conn = nil
			}
			c.connMu.Unlock()
		}()

		// Первоначальное подключение
		for attempt := 0; attempt < maxReconnectAttempts; attempt++ {
			if err := c.ConnectToWebsocket(ctx); err == nil {
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
					if err := c.ConnectToWebsocket(ctx); err == nil {
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
				c.reconnectMsg()
				log.Printf("Периодическое переподключение к вебсокету")
				reconnectTimer.Reset(reconnectInterval)

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
						c.reconnectMsg()
						continue
					}
					log.Printf("Ошибка чтения сообщения: msg %v err %v", msg, err)
					c.reconnectMsg()
					continue
				}
				log.Printf("message received: %v", msg)

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
					} else {
						log.Printf("Ошибка декодирования обновления ордера: %v", err)
					}
				}
			}
		}
	}()

	return nil
}

func (c *MEXCClient) reconnectMsg() {
	select {
	case c.reconnectCh <- struct{}{}:
		// Сигнал отправлен
	default:
		log.Printf("reconnectCh full, skipping signal")
	}
}

func (c *MEXCClient) ping(ctx context.Context) {
	go func() {
		pingTicker := time.NewTicker(pingInterval)
		defer pingTicker.Stop()
		for {
			select {
			case <-pingTicker.C:
				c.connMu.RLock()
				if c.conn != nil {
					c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if err := c.conn.WriteJSON(map[string]interface{}{"method": "PING"}); err != nil {
						log.Printf("Ping failed: %v", err)
						c.reconnectMsg()
					}
					log.Printf("Отправлен пинг на вебсокет ордеров соединение")
				}
				c.connMu.RUnlock()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// ConnectToWebsocket - подключение к WebSocket
// listenKey протухает каждые 60 минут, поэтому его нужно обновлять
func (c *MEXCClient) ConnectToWebsocket(ctx context.Context) error {
	listenKey, err := c.CreateListenKey()
	if err != nil {
		return fmt.Errorf("failed to create listen key: %w", err)
	}
	wsURL := fmt.Sprintf(c.wsURL, listenKey)

	c.connMu.Lock()
	defer c.connMu.Unlock()

	// Устанавливаем соединение
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		log.Printf("failed to connect to WebSocket: %v", err)
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	// Устанавливаем таймауты чтения и записи
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	paramString := "spot@private.orders.v3.api"
	// Подписываемся на обновления ордеров
	subscribeMsg := map[string]interface{}{
		"method": "SUBSCRIPTION",
		"params": []string{paramString},
	}
	if err := conn.WriteJSON(subscribeMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	// Читаем ответ на подписку
	var response map[string]interface{}
	if err := conn.ReadJSON(&response); err != nil {
		log.Printf("failed to read subscription response: %v", err)
		conn.Close()
		return fmt.Errorf("failed to read subscription response: %w", err)
	}
	log.Printf("Subscription response: %v", response)
	if code, ok := response["code"].(float64); ok && (code != 0 || response["msg"] != paramString) {
		log.Printf("subscription failed: %v", response["msg"])
		conn.Close()
		return fmt.Errorf("subscription failed: %v", response["msg"])
	}

	log.Printf("Успешно подписались на обновления ордеров")
	c.conn = conn
	return nil
}

func (c *MEXCClient) CreateListenKey() (string, error) {
	url := c.baseURL + "/api/v3/userDataStream"
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	queryString := fmt.Sprintf("timestamp=%s", timestamp)
	signature := c.sign(queryString) // Предполагается, что метод sign создает HMAC SHA256 подпись

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	q := req.URL.Query()
	q.Add("timestamp", timestamp)
	q.Add("signature", signature)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("X-MEXC-APIKEY", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to create listen key: %s", string(body))
	}

	var result struct {
		ListenKey string `json:"listenKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.ListenKey, nil
}
