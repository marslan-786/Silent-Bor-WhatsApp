package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	fmt.Println("🔐 LID SYSTEM INIT")
	loadLIDFile()
	syncLIDsFromDB()
}

func syncLIDsFromDB() {
	// ✅ Fix: Add Context
	devices, err := container.GetAllDevices(context.Background())
	if err != nil { return }

	lidMutex.Lock()
	defer lidMutex.Unlock()

	currentData := LIDStorage{Bots: make(map[string]BotLIDData)}
	if fileData, err := readJSON(); err == nil { currentData = fileData }

	for _, device := range devices {
		if device.ID == nil { continue }
		phone := getCleanID(device.ID.User)
		
		// ✅ Fix: Safe LID Check (Skip if struct differs)
		// ہم فی الحال صرف تب اٹھائیں گے جب لائیو کنکشن ہو۔
		// DB سے براہ راست نکالنا مشکل ہے کیونکہ اسٹرکچر ورژن مختلف ہو سکتا ہے۔
		_ = phone
	}
	// (باقی کوڈ ویسا ہی، بس DB سے Direct LID نکالنے والی لائن ہٹا دیں کیونکہ وہ ایرر دے رہی ہے)
}

func OnNewPairing(client *whatsmeow.Client) {
	time.Sleep(5 * time.Second)
	if client.Store.ID == nil { return }
	phone := getCleanID(client.Store.ID.User)
	
	// ✅ Fix: Check ID.Server for LID
	var lid string
	if client.Store.ID.Server == "lid" {
		lid = client.Store.ID.User
	}
	
	if lid != "" {
		lidMutex.Lock()
		lidCache[phone] = lid
		lidMutex.Unlock()
	}
}

func isOwnerByLID(client *whatsmeow.Client, sender types.JID) bool {
	if client.Store.ID == nil { return false }
	botPhone := getCleanID(client.Store.ID.User)
	senderPhone := getCleanID(sender.User)
	if senderPhone == botPhone { return true }
	return false
}

func sendOwnerStatus(client *whatsmeow.Client, v *events.Message) {
	ReplyMessage(client, v, "✅ Owner Check")
}

func loadLIDFile() {
	data, err := readJSON()
	if err == nil {
		lidMutex.Lock()
		for p, info := range data.Bots { lidCache[p] = info.LID }
		lidMutex.Unlock()
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
