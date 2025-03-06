package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

var clients = make(map[string]*websocket.Conn)
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s", r.Method, r.URL)

	log.Printf("WS %s", r.URL)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to WebSocket: %v", err)
		return
	}
	// 回复消息
	// conn.WriteMessage(websocket.TextMessage, []byte("Hello, client!"))
	defer conn.Close()

	path := r.URL.Path
	splitted := strings.Split(path, "/")
	id := splitted[1]

	clients[id] = conn
	defer delete(clients, id)

	for {
		messageType, data, err := conn.ReadMessage()
		if err != nil {
			log.Printf("Client %s disconnected: %v", id, err)
			return
		}

		if messageType == websocket.TextMessage {
			log.Printf("Client %s << %s", id, data)

			var message map[string]interface{}
			if err := json.Unmarshal(data, &message); err != nil {
				log.Printf("Failed to parse message: %v", err)
				continue
			}

			destId := message["id"].(string)
			dest, ok := clients[destId]
			if ok {
				message["id"] = id
				data, _ := json.Marshal(message)
				log.Printf("Client %s >> %s", destId, data)
				if err := dest.WriteMessage(websocket.TextMessage, data); err != nil {
					log.Printf("Failed to send message to client %s: %v", destId, err)
				}
			} else {
				log.Printf("Client %s not found", destId)
			}
		}
	}
}

func main() {
	port := "8000"
	hostname := "127.0.0.1"

	http.HandleFunc("/", httpHandler)
	//http.HandleFunc("/ws/", wsHandler)

	log.Printf("Server listening on %s:%s", hostname, port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
