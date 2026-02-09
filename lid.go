package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

const LIDDataFile = "/data/lid_storage.json"

type BotLIDData struct {
	Phone       string    `json:"phone"`
	LID         string    `json:"lid"`
	ExtractedAt time.Time `json:"extracted_at"`
}

type LIDStorage struct {
	LastUpdate time.Time             `json:"last_update"`
	Bots       map[string]BotLIDData `json:"bots"`
}

var (
	lidCache = make(map[string]string)
	lidMutex sync.RWMutex
)

func InitLIDSystem() {
	fmt.Println("🔐 LID SYSTEM INITIALIZING...")
	loadLIDFile()
	// Removed complex DB sync to avoid errors, relying on live pairing now.
}

func OnNewPairing(client *whatsmeow.Client) {
	time.Sleep(5 * time.Second)
	if client.Store.ID == nil { return }
	phone := getCleanID(client.Store.ID.User)
	
	// Check if we are logged in with LID
	var lid string
	if client.Store.ID.Server == "lid" {
		lid = client.Store.ID.User
	}
	
	// If found, save it
	if lid != "" {
		lidMutex.Lock()
		lidCache[phone] = lid
		
		// Update File
		data, _ := readJSON()
		if data.Bots == nil { data.Bots = make(map[string]BotLIDData) }
		data.Bots[phone] = BotLIDData{Phone: phone, LID: lid, ExtractedAt: time.Now()}
		data.LastUpdate = time.Now()
		saveJSON(data)
		
		lidMutex.Unlock()
		fmt.Printf("✅ Saved LID for %s\n", phone)
	}
}

func isOwnerByLID(client *whatsmeow.Client, sender types.JID) bool {
	if client.Store.ID == nil { return false }
	botPhone := getCleanID(client.Store.ID.User)
	senderPhone := getCleanID(sender.User)
	
	// Simple check: Is the sender the bot itself? (Self-message)
	if senderPhone == botPhone { return true }
	
	// Cache Check
	lidMutex.RLock()
	cachedLID, exists := lidCache[botPhone]
	lidMutex.RUnlock()
	
	if exists && getCleanID(sender.User) == getCleanID(cachedLID) {
		return true
	}
	
	return false
}

func sendOwnerStatus(client *whatsmeow.Client, v *events.Message) {
	ReplyMessage(client, v, "✅ Owner System Active")
}

func loadLIDFile() {
	data, err := readJSON()
	if err == nil {
		lidMutex.Lock()
		for p, info := range data.Bots {
			lidCache[p] = info.LID
		}
		lidMutex.Unlock()
		fmt.Printf("📂 Loaded %d LIDs.\n", len(data.Bots))
	}
}

func readJSON() (LIDStorage, error) {
	var data LIDStorage
	file, err := os.ReadFile(LIDDataFile)
	if err != nil { return data, err }
	json.Unmarshal(file, &data)
	return data, nil
}

func saveJSON(data LIDStorage) {
	dir := filepath.Dir(LIDDataFile)
	os.MkdirAll(dir, 0755)
	bytes, _ := json.MarshalIndent(data, "", "  ")
	os.WriteFile(LIDDataFile, bytes, 0644)
}
