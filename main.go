package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events" // ✅ NEW IMPORT ADDED
	waLog "go.mau.fi/whatsmeow/util/log"
)

// ⚙️ CONSTANTS & PATHS
const (
	VolumeDir    = "/data" // Railway Persistent Volume
	DBName       = "sessions.db"
	SettingsFile = "settings.json"
	Port         = "8080"
)

// 🌍 GLOBAL VARIABLES
var (
	sm = &SessionManager{
		Clients:  make(map[string]*whatsmeow.Client),
		Settings: make(map[string]*BotSettings),
	}
	container *sqlstore.Container
	upgrader  = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	wsClients = make(map[*websocket.Conn]bool)
	wsMutex   sync.Mutex
)

// ==========================================
// 🚀 MAIN FUNCTION
// ==========================================
func main() {
	log.Println("🚀 STARTING SYSTEM | FIXED EVENTS...")

	if _, err := os.Stat(VolumeDir); os.IsNotExist(err) {
		_ = os.Mkdir(VolumeDir, 0755)
	}

	initDB()
	InitLIDSystem() // Initialize LID System
	loadSettings()
	restoreSessions()

	go autoSaveLoop()
	setupRoutes()
	startServer()
}

// ==========================================
// 🛠️ INITIALIZATION HELPERS
// ==========================================

func initDB() {
	dbPath := filepath.Join(VolumeDir, DBName)
	dbLog := waLog.Stdout("Database", "ERROR", true)
	var err error
	container, err = sqlstore.New(context.Background(), "sqlite3", "file:"+dbPath+"?_foreign_keys=on", dbLog)
	if err != nil {
		log.Fatalf("❌ SQLite Init Failed: %v", err)
	}
	if err = container.Upgrade(context.Background()); err != nil {
		log.Fatalf("❌ DB Upgrade Failed: %v", err)
	}
}

func setupRoutes() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "index.html")
	})
	http.HandleFunc("/pic.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "pic.png")
	})
	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/api/pair", handlePair)
}

func startServer() {
	server := &http.Server{Addr: ":" + Port}
	
	go func() {
		fmt.Printf("🌐 Server Live on Port %s\n", Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server Error: %v", err)
		}
	}()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	fmt.Println("\n🛑 Shutting down...")
	saveSettings()
	sm.mu.Lock()
	for _, client := range sm.Clients {
		client.Disconnect()
	}
	sm.mu.Unlock()
	fmt.Println("👋 Goodbye!")
}

// ==========================================
// 🔌 LOGIC HANDLERS
// ==========================================

func restoreSessions() {
	devices, err := container.GetAllDevices(context.Background())
	if err != nil { return }

	fmt.Printf("🔄 Restoring %d Sessions...\n", len(devices))
	for _, device := range devices {
		botID := getCleanID(device.ID.User)
		go connectBot(device, botID)
	}
}

func connectBot(device *store.Device, botID string) {
	sm.mu.Lock()
	if _, exists := sm.Clients[botID]; exists {
		sm.mu.Unlock()
		return
	}
	
	if _, ok := sm.Settings[botID]; !ok {
		sm.Settings[botID] = &BotSettings{Prefix: ".", AlwaysOnline: true}
	}
	sm.mu.Unlock()

	client := whatsmeow.NewClient(device, waLog.Stdout("Client", "ERROR", true))
	
	// 🔥 EVENT HANDLER FIXED HERE
	client.AddEventHandler(func(evt interface{}) {
		HandleMessages(client, evt) // From commands.go
		
		// ✅ FIX: Correct Type Assertion
		if _, ok := evt.(*events.PairStatus); ok {
			// Save LID on pairing success
			OnNewPairing(client)
		}
	})

	if err := client.Connect(); err != nil {
		fmt.Printf("❌ Failed to connect %s: %v\n", botID, err)
		return
	}

	sm.mu.Lock()
	sm.Clients[botID] = client
	sm.mu.Unlock()
	
	fmt.Printf("✅ Bot Online: %s\n", botID)
	broadcastWS(WSMessage{Type: "new_session", BotID: botID})
	
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			if client.IsConnected() && sm.Settings[botID].AlwaysOnline {
				client.SendPresence(context.Background(), types.PresenceAvailable)
			}
		}
	}()
}

// ==========================================
// 💾 PERSISTENCE LOGIC
// ==========================================

func loadSettings() {
	path := filepath.Join(VolumeDir, SettingsFile)
	data, err := os.ReadFile(path)
	if err == nil {
		sm.mu.Lock()
		json.Unmarshal(data, &sm.Settings)
		sm.mu.Unlock()
		fmt.Println("📂 Settings Loaded.")
	}
}

func saveSettings() {
	sm.mu.RLock()
	data, err := json.MarshalIndent(sm.Settings, "", "  ")
	sm.mu.RUnlock()
	if err == nil {
		os.WriteFile(filepath.Join(VolumeDir, SettingsFile), data, 0644)
	}
}

func autoSaveLoop() {
	for range time.Tick(30 * time.Second) {
		saveSettings()
	}
}

// ==========================================
// 🌐 API HANDLERS
// ==========================================

func handlePair(w http.ResponseWriter, r *http.Request) {
	var req PairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	number := strings.ReplaceAll(req.Number, "+", "")
	cleanID := getCleanID(number)
	
	device := container.NewDevice()
	client := whatsmeow.NewClient(device, waLog.Stdout("Pairing", "INFO", true))
	
	if err := client.Connect(); err != nil {
		http.Error(w, "Connection Failed", 500)
		return
	}

	code, err := client.PairPhone(context.Background(), number, true, whatsmeow.PairClientChrome, "Linux")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				client.Disconnect()
				return
			default:
				if client.Store.ID != nil {
					connectBot(device, cleanID)
					return
				}
				time.Sleep(1 * time.Second)
			}
		}
	}()

	json.NewEncoder(w).Encode(map[string]string{"code": code})
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil { return }
	
	wsMutex.Lock()
	wsClients[conn] = true
	wsMutex.Unlock()

	sm.mu.RLock()
	active := len(sm.Clients)
	sm.mu.RUnlock()
	
	conn.WriteJSON(WSMessage{Type: "stats", ActiveBots: active})

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			wsMutex.Lock()
			delete(wsClients, conn)
			wsMutex.Unlock()
			break
		}
	}
}

func broadcastWS(msg WSMessage) {
	wsMutex.Lock()
	defer wsMutex.Unlock()
	for conn := range wsClients {
		conn.WriteJSON(msg)
	}
}

func getCleanID(s string) string {
	if strings.Contains(s, ":") {
		return strings.Split(s, ":")[0]
	}
	return strings.Split(s, "@")[0]
}
