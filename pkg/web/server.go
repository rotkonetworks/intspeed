package web

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rotkonetworks/intspeed/pkg/locations"
	"github.com/rotkonetworks/intspeed/pkg/speedtest"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	clients map[*websocket.Conn]bool
	mu      sync.RWMutex
}

type Message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

func NewServer() *Server {
	return &Server{
		clients: make(map[*websocket.Conn]bool),
	}
}

func (s *Server) addClient(conn *websocket.Conn) {
	s.mu.Lock()
	s.clients[conn] = true
	s.mu.Unlock()
}

func (s *Server) removeClient(conn *websocket.Conn) {
	s.mu.Lock()
	delete(s.clients, conn)
	s.mu.Unlock()
	conn.Close()
}

func (s *Server) broadcast(msg Message) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	data, _ := json.Marshal(msg)
	for client := range s.clients {
		if err := client.WriteMessage(websocket.TextMessage, data); err != nil {
			client.Close()
			delete(s.clients, client)
		}
	}
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	s.addClient(conn)
	defer s.removeClient(conn)

	// Send initial data
	s.sendLocations(conn)

	for {
		var msg Message
		if err := conn.ReadJSON(&msg); err != nil {
			break
		}

		switch msg.Type {
		case "start_test":
			go s.runTest(conn)
		}
	}
}

func (s *Server) sendLocations(conn *websocket.Conn) {
	msg := Message{
		Type: "locations",
		Data: locations.GlobalLocations,
	}
	conn.WriteJSON(msg)
}

func (s *Server) runTest(conn *websocket.Conn) {
	client := speedtest.NewClient(speedtest.Config{
		Threads: 1,
	})

	msg := Message{Type: "test_started"}
	conn.WriteJSON(msg)

	for i, location := range locations.GlobalLocations {
		msg := Message{
			Type: "test_progress",
			Data: map[string]interface{}{
				"current": i + 1,
				"total":   len(locations.GlobalLocations),
				"location": location.Name,
			},
		}
		conn.WriteJSON(msg)

		result := client.TestLocation(context.Background(), location)
		
		msg = Message{
			Type: "test_result",
			Data: result,
		}
		conn.WriteJSON(msg)
	}

	msg = Message{Type: "test_complete"}
	conn.WriteJSON(msg)
}

func (s *Server) Routes() *mux.Router {
	r := mux.NewRouter()
	
	r.HandleFunc("/ws", s.handleWebSocket)
	r.HandleFunc("/", s.serveIndex).Methods("GET")
	
	return r
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Interactive Speedtest</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body class="bg-gray-50 min-h-screen">
    <div class="container mx-auto px-4 py-8">
        <div class="bg-white rounded-lg shadow-lg p-6 mb-8">
            <h1 class="text-4xl font-bold text-gray-800 mb-2">🌍 Interactive Global Speedtest</h1>
            <p class="text-gray-600">Real-time network performance testing</p>
        </div>

        <div class="bg-white rounded-lg p-6 shadow mb-8">
            <div class="flex items-center justify-between mb-4">
                <h2 class="text-2xl font-semibold">Test Control</h2>
                <button id="startTest" class="bg-blue-600 text-white px-6 py-2 rounded-lg hover:bg-blue-700 disabled:opacity-50" onclick="startTest()">
                    Start Global Test
                </button>
            </div>
            
            <div id="progress" class="hidden">
                <div class="mb-2">
                    <div class="flex justify-between text-sm">
                        <span id="progressText">Preparing...</span>
                        <span id="progressCount">0/0</span>
                    </div>
                    <div class="w-full bg-gray-200 rounded-full h-2">
                        <div id="progressBar" class="bg-blue-600 h-2 rounded-full transition-all duration-300" style="width: 0%"></div>
                    </div>
                </div>
            </div>
        </div>

        <div class="bg-white rounded-lg p-6 shadow">
            <h3 class="text-xl font-semibold mb-4">Live Results</h3>
            <div id="results" class="space-y-2"></div>
        </div>
    </div>

    <script>
        let ws;
        let results = [];
        
        function connectWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            ws = new WebSocket(protocol + '//' + window.location.host + '/ws');
            
            ws.onmessage = function(event) {
                const msg = JSON.parse(event.data);
                handleMessage(msg);
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };
        }
        
        function handleMessage(msg) {
            switch(msg.type) {
                case 'locations':
                    console.log('Locations received:', msg.data.length);
                    break;
                case 'test_started':
                    document.getElementById('progress').classList.remove('hidden');
                    document.getElementById('startTest').disabled = true;
                    document.getElementById('results').innerHTML = '';
                    results = [];
                    break;
                case 'test_progress':
                    const progress = (msg.data.current / msg.data.total) * 100;
                    document.getElementById('progressBar').style.width = progress + '%';
                    document.getElementById('progressText').textContent = 'Testing ' + msg.data.location + '...';
                    document.getElementById('progressCount').textContent = msg.data.current + '/' + msg.data.total;
                    break;
                case 'test_result':
                    results.push(msg.data);
                    addResult(msg.data);
                    break;
                case 'test_complete':
                    document.getElementById('progressText').textContent = 'Complete!';
                    document.getElementById('startTest').disabled = false;
                    break;
            }
        }
        
        function addResult(result) {
            const resultsDiv = document.getElementById('results');
            const div = document.createElement('div');
            div.className = 'flex justify-between items-center p-3 border rounded-lg ' + 
                (result.success ? 'border-green-200 bg-green-50' : 'border-red-200 bg-red-50');
            
            div.innerHTML = ` + "`" + `
                <div>
                    <div class="font-medium">${result.location.name}</div>
                    <div class="text-sm text-gray-600">${result.location.region}</div>
                </div>
                <div class="text-right">
                    ${result.success ? 
                        ` + "`" + `<div class="text-sm">
                            <div>📶 ${result.latency_ms.toFixed(1)}ms</div>
                            <div>⬇️ ${result.download_mbps.toFixed(1)} Mbps</div>
                            <div>⬆️ ${result.upload_mbps.toFixed(1)} Mbps</div>
                        </div>` + "`" + ` :
                        ` + "`" + `<div class="text-red-600 text-sm">${result.error}</div>` + "`" + `
                    }
                </div>
            ` + "`" + `;
            
            resultsDiv.appendChild(div);
            resultsDiv.scrollTop = resultsDiv.scrollHeight;
        }
        
        function startTest() {
            if (ws && ws.readyState === WebSocket.OPEN) {
                ws.send(JSON.stringify({type: 'start_test'}));
            }
        }
        
        // Initialize
        connectWebSocket();
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}
