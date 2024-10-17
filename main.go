package main

import (
	"log"
	"nomarztest/binance"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
)

func handleInterrupt(c binance.OrderBookClient) {
	log.Println("interrupt, closing down connection")
	// Cleanly close the connection by sending a close message and then
	// waiting (with timeout) for the server to close the connection.
	err := c.Ws.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	if err != nil {
		log.Println("write close:", err)
		return
	}
	select {
	case <-c.Done:
	case <-time.After(time.Second):
	}
}

func main() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	c := binance.CreateOrderBookClient("BTCUSDT")
	c.SetTimeout(15 * time.Second)
	err := c.Connect()
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("Connected to Binance WS")
	c.Start()

	server := CreateWSServer(c)
	server.Start()

	// wait for server to starts before listening for disconnections
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Done:
			log.Println("Disconnected from Binance WS")
			return
		case <-interrupt:
			handleInterrupt(c)
			return
		}
	}
}
