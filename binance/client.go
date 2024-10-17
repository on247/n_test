package binance

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// represent a single price/qty pair on the order book
type OrderBookEntry struct {
	price float64
	qty   float64
}

// Custom unmarshaler required to parse the array that makes up each order into a struck
func (o *OrderBookEntry) UnmarshalJSON(data []byte) error {
	// Ignore null, like in the main JSON package.
	if string(data) == "null" || string(data) == `""` {
		return nil
	}
	var content []string
	err := json.Unmarshal(data, &content)
	if err != nil {
		return err
	}
	price, err := strconv.ParseFloat(content[0], 64)
	if err != nil {
		return err
	}
	qty, err := strconv.ParseFloat(content[1], 64)
	if err != nil {
		return err
	}
	entry := &OrderBookEntry{
		price: price,
		qty:   qty,
	}
	*o = *entry
	return err
}

type OrderBook struct {
	// orders are indexed by price.
	Asks map[float64]float64
	Bids map[float64]float64
}

// Struct for parsing of event received from the Binance WS stream.
type DepthUpdate struct {
	EventName     string           `json:"e"`
	Time          int64            `json:"E"`
	Symbol        string           `json:"S"`
	FirstUpdateID int64            `json:"U"`
	LastUpdateID  int64            `json:"u"`
	Asks          []OrderBookEntry `json:"a"`
	Bids          []OrderBookEntry `json:"b"`
}

// Represent a instance of the Client connecting to WS
type OrderBookClient struct {
	Ws        *websocket.Conn // Underlying websock
	timeout   time.Duration   // Connection timeout
	pair      string          // Trading Pait
	orders    *OrderBook      // last view (cache) of the order book
	AvgUpdate chan float64    // channel used to send price updates to
	Done      chan struct{}   // channel to notify disconnection
}

// Update cached orderbook.
func (c *OrderBookClient) updateOrderBook(update *DepthUpdate) {
	// simply replace the orders or insert new ones , trivial since they are indexed by price.
	for _, item := range update.Asks {
		c.orders.Asks[item.price] = item.qty
	}
	for _, item := range update.Bids {
		c.orders.Bids[item.price] = item.qty
	}
}

func (c *OrderBookClient) calculateAvg() float64 {
	var bidSum float64 = 0.0
	var askSum float64 = 0.0
	// add up bids and asks
	for price := range c.orders.Asks {
		bidSum += price
	}
	for price := range c.orders.Bids {
		askSum += price
	}
	// avg = bids + asks / amount of orders.
	orderCount := len(c.orders.Asks) + len(c.orders.Bids)
	orderSum := bidSum + askSum
	avg := orderSum / float64(orderCount)
	return avg
}

// process a raw message from the stream server
func (c *OrderBookClient) processUpdate(message []byte) {
	update := &DepthUpdate{}
	// parse message
	err := json.Unmarshal(message, update)
	if err != nil {
		log.Println("error parsing update:", err)
		return
	}
	// update orderbook , calculate avg, and send out update.
	c.updateOrderBook(update)
	c.AvgUpdate <- c.calculateAvg()
}

func (c *OrderBookClient) SetTimeout(t time.Duration) {
	c.timeout = t
}

func (c *OrderBookClient) Connect() error {
	var URL = "wss://stream.binance.com:9443/ws/" + strings.ToLower(c.pair) + "@depth"
	ws, _, err := websocket.DefaultDialer.Dial(URL, nil)
	if err != nil {
		log.Fatal("dial:", err)
		return err
	}
	c.Ws = ws
	return nil
}

func (c *OrderBookClient) Start() {
	go func() {
		for {
			c.Ws.SetReadDeadline(time.Now().Add(c.timeout))
			_, message, err := c.Ws.ReadMessage()
			if err != nil {
				log.Println("read error:", err)
				close(c.Done)
				return
			}
			c.processUpdate(message)
		}
	}()
}
func CreateOrderBookClient(pair string) OrderBookClient {
	orders := &OrderBook{
		Asks: map[float64]float64{},
		Bids: map[float64]float64{},
	}

	done := make(chan struct{})
	avgUpdate := make(chan float64)

	client := &OrderBookClient{
		Ws:        nil,
		pair:      pair,
		Done:      done,
		AvgUpdate: avgUpdate,
		orders:    orders,
	}
	return *client
}
