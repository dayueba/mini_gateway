package internal

import (
	"github.com/goccy/go-json"
	"github.com/gorilla/websocket"
	"log"
	"net/url"
	"sync"
	"testing"
)

func TestConcurrencyConnectServer(t *testing.T) {
	var wg sync.WaitGroup
	count := 10
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func() {
			for {
				connectToServer()
				wg.Done()
			}
		}()
	}

	wg.Wait()
}

func TestConnectServer(t *testing.T) {
	connectToServer()
}

func connectToServer() {
	u := url.URL{Scheme: "ws", Host: "localhost:7100", Path: "/connect"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return
	}
	msg := BizMessage{
		Type: "PING",
		Data: nil,
	}
	data, _ := json.Marshal(msg)
	err = c.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		panic(err)
	}
	// 循环读消息
	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}
		log.Printf("recv: %s", message)
	}
	c.Close()
}
