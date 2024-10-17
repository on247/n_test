package main

import (
	"log"
	"net/http"
	"nomarztest/binance"
	"strconv"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type TickerServer struct {
	lastAvg float64
	addr    string
	binance binance.OrderBookClient
	clients []*websocket.Conn
}

func (t *TickerServer) ticker(w http.ResponseWriter, r *http.Request) {
	// upgrade to WS
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer c.Close()
	// save connection to the client, use to broadcast later.
	t.clients = append(t.clients, c)
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
		// always reply with the last value seen no matter the content of the message.
		// parse float
		value := strconv.FormatFloat(t.lastAvg, 'f', -1, 64)
		err = c.WriteMessage(mt, []byte(value))
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func (t *TickerServer) broadcastTickerUpdate(value float64) {
	// iterate list of clients and broadcast latest ticker update
	for _, c := range t.clients {
		value := strconv.FormatFloat(value, 'f', -1, 64)
		log.Println("clients:", len(t.clients))
		c.WriteMessage(1, []byte(value))
	}
}

func (t *TickerServer) Start() {
	go func() {
		// register handler for ws connection endpoint.
		http.HandleFunc("/ws/ticker", t.ticker)
		http.ListenAndServe(t.addr, nil)
		// listen to the AvgUpdate channels for updates from the binance order client.
		for {
			newAvg := <-t.binance.AvgUpdate
			t.lastAvg = newAvg
			// push new value to websocket
			t.broadcastTickerUpdate(newAvg)
		}
	}()
}

func CreateWSServer(binance binance.OrderBookClient) TickerServer {
	server := &TickerServer{
		lastAvg: 0.0,
		addr:    "localhost:8080",
		binance: binance,
	}
	return *server
}
