package exchange

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"io"
	"log"
	"net/http"
	"time"
)

// OrderStatus представляет статус ордера.
type OrderStatus int32

const (
	NotTraded         OrderStatus = 1 // Не исполнен
	FullyTraded       OrderStatus = 2 // Полностью исполнен
	PartiallyTraded   OrderStatus = 3 // Частично исполнен
	Canceled          OrderStatus = 4 // Отменён
	PartiallyCanceled OrderStatus = 5 // Частично отменён
)

type subscribeMsg struct {
	Method string   `json:"method"`
	Params []string `json:"params"`
}

// OrderUpdate - структура для обновлений ордеров
type OrderUpdate struct {
	OrderId string
	Price   string
	Status  int32
	//Общее количество (base asset), которое уже исполнено в рамках данного ордера.
	//В KAS для пары KAS/USDT.
	Quantity        string
	CreateTimestamp int64
}

// Таймеры для периодического обновления соединения и пинга
const reconnectInterval = 15 * time.Minute
const pingInterval = 15 * time.Second
const maxReconnectAttempts = 5

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
			} else {
				c.logger.Error(fmt.Sprintf("Ошибка подключения к вебсокету: %v попытка: %d", err, attempt))
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
					} else {
						c.logger.Error(fmt.Sprintf("Ошибка подключения к вебсокету: %v попытка: %d", err, attempt))
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

				// чтение сообщения
				c.conn.SetReadDeadline(time.Now().Add(time.Minute))
				msgType, msg, err := c.conn.ReadMessage()
				c.connMu.RUnlock()
				if err != nil {
					if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
						c.logger.Error(fmt.Sprintf("Чтение из закрытого соединения: %v", err))
						log.Printf("Читаем из закрытого соедининия: %v", err)
						c.reconnectMsg()
						continue
					}
					c.logger.Error(fmt.Sprintf("Ошибка чтения сообщения: msg %v msgType: %d err %v", string(msg), msgType, err))
					log.Printf("Ошибка чтения сообщения: msg %v msgType: %d err %v", string(msg), msgType, err)
					c.reconnectMsg()
					continue
				}
				//if msgType == websocket.TextMessage {
				//	log.Printf("message received: %v msgType: %d", string(msg), msgType)
				//}

				// Десериализация Protobuf
				if msgType == websocket.BinaryMessage {
					var wsMessage PrivateOrdersV3Api
					if err := proto.Unmarshal(msg, &wsMessage); err != nil {
						c.logger.Error(fmt.Sprintf("Protobuf unmarshal error: %v", err))
						log.Printf("Protobuf unmarshal error: %v", err)
						continue
					}
					//log.Printf("Message received: %v msgType: %d", string(msg), msgType)

					// Обработка обновлений ордеров
					update := OrderUpdate{
						OrderId:         wsMessage.GetPrivateOrders().GetId(),
						Price:           wsMessage.GetPrivateOrders().GetPrice(),
						CreateTimestamp: wsMessage.GetPrivateOrders().GetCreateTime(),
						Status:          wsMessage.GetPrivateOrders().GetStatus(),
						Quantity:        wsMessage.GetPrivateOrders().GetCumulativeAmount(),
					}
					select {
					case updateCh <- update:
					case <-ctx.Done():
						return
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
						c.logger.Error(fmt.Sprintf("Ping error: %v", err))
						log.Printf("Ping failed: %v", err)
						c.reconnectMsg()
					}
					//log.Printf("Отправлен пинг на вебсокет ордеров соединение")
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

	paramString := "spot@private.orders.v3.api.pb"
	// Подписываемся на обновления ордеров
	msg := subscribeMsg{
		Method: "SUBSCRIPTION",
		Params: []string{paramString},
	}
	msgData, err := json.Marshal(msg)
	if err != nil {
		log.Printf("failed to json Marshal msgData: %v", err)
		conn.Close()
		return fmt.Errorf("failed to json Marshal msgData: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, msgData); err != nil {
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
	//log.Printf("Subscription response: %v", response)
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
