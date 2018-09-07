package main

import (
	"net/http"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"time"
	"im/libs/proto"
	"encoding/json"
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)
func serveHome(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL)
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, "home.html")
}

func InitWebsocket(bind string) (err error) {
	log.Infof("size: %d",DefaultServer.Options.ReadBufferSize)



	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {

		serveWs(DefaultServer, w, r)
	})


	err = http.ListenAndServe(bind, nil)

	return err

}

// serveWs handles websocket requests from the peer.
func serveWs(server *Server, w http.ResponseWriter, r *http.Request) {
	// upgrader := websocket.Upgrader{
	// 	ReadBufferSize:  DefaultServer.Options.ReadBufferSize,
	// 	WriteBufferSize: DefaultServer.Options.WriteBufferSize,
	// }


	var upgrader = websocket.Upgrader{
		ReadBufferSize:  DefaultServer.Options.ReadBufferSize,
		WriteBufferSize: DefaultServer.Options.WriteBufferSize,
	}


	conn, err := upgrader.Upgrade(w, r, nil)

	if err != nil {
		log.Error(err)
		return
	}


	go server.writePump(conn)
	go server.readPump(conn)
}



func (s *Server) readPump(conn *websocket.Conn) {
	defer func() {
		conn.Close()
	}()

	conn.SetReadLimit(s.Options.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(s.Options.PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(s.Options.PongWait));
		return nil
	})

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway,websocket.CloseAbnormalClosure) {
				log.Errorf("readPump ReadMessage err:%v", err)
			}
		}
		var connArg  proto.ConnArg
		log.Infof("message :%s", message)
		if err := json.Unmarshal([]byte(message), &connArg); err != nil  {
			log.Errorf("message struct %b", connArg)
		}

		b := s.Bucket(connArg.Key)
		b.broadcast <- message
		ch := new(Channel)
		ch.conn = conn
		err = b.Put(connArg.Key, connArg.RoomId, ch)
		if err != nil {
			conn.Close()
		}


	}
}

func (s *Server) writePump(conn *websocket.Conn) {
	var key = "e4bac70ea5d10dfb"
	b := s.Bucket(key)

	ticker := time.NewTicker(s.Options.PingPeriod)
	log.Printf("ticker :%v", ticker)


	defer func() {
		ticker.Stop()
		conn.Close()
	}()
	for {
		select {
		case message, ok := <-b.broadcast:
			conn.SetWriteDeadline(time.Now().Add(s.Options.WriteWait))
			if !ok {
				// The hub closed the channel.
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			log.Printf("TextMessage :%v", websocket.TextMessage)
			w, err := conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			log.Printf("message :%v", message)
			w.Write(message)
			// Add queued chat messages to the current websocket message.
			n := len(b.broadcast)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-b.broadcast)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(s.Options.WriteWait))
			log.Printf("websocket.PingMessage :%v", websocket.PingMessage)
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}



