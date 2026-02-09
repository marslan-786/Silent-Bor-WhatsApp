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

// ⚙️ CONSTANTS
const (
	LIDDataFile = "/data/lid_storage.json" // Permanent Volume Path
)

// 📦 DATA STRUCTURES
type BotLIDData struct {
	Phone       string    `json:"phone"`
	LID         string    `json:"lid"`
	ExtractedAt time.Time `json:"extracted_at"`
}

type LIDStorage struct {
	LastUpdate time.Time             `json:"last_update"`
	Bots       map[string]BotLIDData `json:"bots"` // Phone -> Data
}

// 🔒 GLOBAL CACHE & MUTEX
var (
	lidCache = make(map[string]string) // Fast RAM Access: Phone -> LID
	lidMutex sync.RWMutex
)

// ==========================================
// 🚀 INITIALIZATION (ENTRY POINT)
// ==========================================

// Main.go میں InitDB کے فوراً بعد اسے کال کریں
func InitLIDSystem() {
	fmt.Println("\n╔═══════════════════════════════════════╗")
	fmt.Println("║   🔐 LID SYSTEM INITIALIZING (LOCAL)  ║")
	fmt.Println("╚═══════════════════════════════════════╝\n")

	// 1. Load existing data from JSON
	loadLIDFile()

	// 2. Extract fresh data from SQLite DB
	syncLIDsFromDB()
}

// ==========================================
// 🔄 CORE LOGIC: EXTRACT & SYNC
// ==========================================

// یہ فنکشن براہ راست آپ کے SQLite سیشنز سے LID نکالتا ہے
func syncLIDsFromDB() {
	fmt.Println("🔍 Scanning Session Database for LIDs...")

	// کنٹینر (جو main.go میں بنا ہے) سے تمام ڈیوائسز لیں
	devices, err := container.GetAllDevices()
	if err != nil {
		fmt.Printf("⚠️ Failed to read sessions: %v\n", err)
		return
	}

	lidMutex.Lock()
	defer lidMutex.Unlock()

	updates := 0
	
	// اسٹرکچر تیار کریں اگر خالی ہے
	currentData := LIDStorage{
		Bots: make(map[string]BotLIDData),
	}
	// پرانا ڈیٹا لوڈ کریں تاکہ مکس ہو سکے
	if fileData, err := readJSON(); err == nil {
		currentData = fileData
	}

	// ہر ڈیوائس کو چیک کریں
	for _, device := range devices {
		if device.ID == nil { continue }

		// فون نمبر اور LID نکالیں
		phone := getCleanID(device.ID.User)
		
		// ⚠️ اہم: WhatsMeow Store میں LID اکثر `device.RegistrationId` یا `Account` میں ہوتا ہے
		// لیکن سب سے بہترین طریقہ یہ ہے کہ ہم ID.User اور ID.Server چیک کریں
		// اگر ID.Server "lid" ہے تو وہ LID ہے، ورنہ ہمیں ڈیوائس کے اندر LID فیلڈ ڈھونڈنی ہوگی
		// چونکہ WhatsMeow SQLStore میں LID الگ کالم میں نہیں ہوتا، یہ `SignalProtocolStore` میں ہوتا ہے۔
		// لیکن ایک آسان طریقہ یہ ہے کہ ہم `device.ID` (Phone) اور اس کے ساتھ جڑی `Identity` چیک کریں۔
		// بہرحال، سادہ ترین حل یہ ہے:
		
		var lid string
		// اگر ڈیوائس کے پاس LID محفوظ ہے (اکثر ایڈوانس سیشنز میں ہوتا ہے)
		if device.Account != nil && device.Account.LID != "" {
			lid = getCleanID(device.Account.LID)
		} else {
			// اگر LID نہیں ملا تو ہم فی الحال اسے چھوڑ دیتے ہیں
			// جب بوٹ کنیکٹ ہوگا تو `OnNewPairing` میں اسے دوبارہ پکڑ لیں گے
			continue
		}

		if lid != "" {
			// RAM Cache Update
			lidCache[phone] = lid
			
			// JSON Data Update
			currentData.Bots[phone] = BotLIDData{
				Phone:       phone,
				LID:         lid,
				ExtractedAt: time.Now(),
			}
			updates++
			fmt.Printf("✅ Found: %s -> %s\n", phone, lid)
		}
	}

	// اگر کچھ نیا ملا تو فائل سیو کریں
	if updates > 0 {
		currentData.LastUpdate = time.Now()
		saveJSON(currentData)
		fmt.Printf("💾 Synced %d LIDs to Volume.\n", updates)
	} else {
		fmt.Println("💤 No new LIDs found in DB.")
	}
}

// جب بھی نیا بوٹ پیئر ہو، اسے کال کریں
func OnNewPairing(client *whatsmeow.Client) {
	time.Sleep(5 * time.Second) // سیشن سیٹل ہونے کا انتظار کریں

	if client.Store.ID == nil { return }
	
	phone := getCleanID(client.Store.ID.User)
	
	// کلائنٹ سے براہ راست LID مانگیں (یہ سب سے پکا طریقہ ہے)
	// WhatsMeow کنکشن کے دوران LID خود اٹھا لیتا ہے
	var lid string
	
	// طریقہ 1: سٹور سے چیک کریں
	if client.Store.Account != nil {
		lid = getCleanID(client.Store.Account.LID)
	}
	
	// طریقہ 2: اگر سٹور خالی ہے (کبھی کبھار ہوتا ہے)، تو ہم اسے خود کو میسج بھیج کر چیک کر سکتے ہیں (Optional)
	
	if lid != "" {
		fmt.Printf("🆕 New Bot Paired: %s (LID: %s)\n", phone, lid)
		
		lidMutex.Lock()
		lidCache[phone] = lid
		
		// فائل اپڈیٹ کریں
		data, _ := readJSON()
		if data.Bots == nil { data.Bots = make(map[string]BotLIDData) }
		
		data.Bots[phone] = BotLIDData{
			Phone:       phone,
			LID:         lid,
			ExtractedAt: time.Now(),
		}
		data.LastUpdate = time.Now()
		saveJSON(data)
		lidMutex.Unlock()
	} else {
		fmt.Printf("⚠️ Could not extract LID for %s immediately. Will retry on sync.\n", phone)
	}
}

// ==========================================
// 🔐 OWNER VERIFICATION LOGIC
// ==========================================

func isOwnerByLID(client *whatsmeow.Client, sender types.JID) bool {
	// 1. Bot کا فون نمبر نکالیں
	if client.Store.ID == nil { return false }
	botPhone := getCleanID(client.Store.ID.User)

	// 2. Cache سے Bot کی LID نکالیں
	lidMutex.RLock()
	botLID, exists := lidCache[botPhone]
	lidMutex.RUnlock()

	// اگر LID کیشے میں نہیں ہے تو Sync چلائیں
	if !exists {
		syncLIDsFromDB()
		lidMutex.RLock()
		botLID, exists = lidCache[botPhone]
		lidMutex.RUnlock()
		
		if !exists {
			// اگر اب بھی نہیں ملی تو پرانا طریقہ (نمبر میچنگ) استعمال کریں
			return getCleanID(sender.User) == botPhone
		}
	}

	// 3. Sender کا نمبر نکالیں
	senderPhone := getCleanID(sender.User)

	// 4. موازنہ کریں: کیا Sender کا نمبر Bot کی LID کے برابر ہے؟
	// نوٹ: واٹس ایپ میں، جب آپ خود کو میسج کرتے ہیں یا دوسرے ڈیوائس سے، 
	// تو Sender ID اکثر آپ کی اپنی LID ہوتی ہے۔
	
	// Case A: Sender is LID (e.g. 12345:2@lid)
	if strings.Contains(sender.Server, "lid") {
		return getCleanID(sender.User) == getCleanID(botLID)
	}

	// Case B: Sender is Phone (Normal) - But we need to match against Owner's Phone
	// چونکہ LID دراصل Owner کی ہی ایک ID ہے، اس لیے ہمیں یہ دیکھنا ہے کہ
	// کیا Sender وہی بندہ ہے جس کا یہ بوٹ ہے؟
	
	// آسان ترین حل:
	// اگر Sender کا فون نمبر == Bot کا فون نمبر (Self Message)
	if senderPhone == botPhone {
		return true
	}
	
	// اگر Sender کی LID == Bot کی LID (Linked Device Message)
	// (یہ تب کام کرے گا جب Sender ID LID فارمیٹ میں ہو)
	if getCleanID(sender.User) == getCleanID(botLID) {
		return true
	}

	return false
}

// کمانڈ ٹیسٹنگ کے لیے
func sendOwnerStatus(client *whatsmeow.Client, v *events.Message) {
	botPhone := getCleanID(client.Store.ID.User)
	
	lidMutex.RLock()
	lid := lidCache[botPhone]
	lidMutex.RUnlock()
	
	sender := getCleanID(v.Info.Sender.User)
	isOwn := isOwnerByLID(client, v.Info.Sender)
	
	status := "❌ ACCESS DENIED"
	if isOwn { status = "✅ ACCESS GRANTED" }

	msg := fmt.Sprintf(`
╔════════════════════╗
║ 🔐 OWNER DEBUG INFO
╠════════════════════╣
║ 🤖 Bot: %s
║ 🆔 Known LID: %s
║ 👤 Sender: %s
║ 🏷️ Type: %s
╠════════════════════╣
║ %s
╚════════════════════╝
`, botPhone, lid, sender, v.Info.Sender.Server, status)

	// (ReplyMessage function commands.go سے استعمال ہوگا)
	ReplyMessage(client, v, msg)
}

// ==========================================
// 📂 FILE HANDLING (JSON)
// ==========================================

func loadLIDFile() {
	data, err := readJSON()
	if err != nil {
		fmt.Println("⚠️ No LID file found, starting fresh.")
		return
	}

	lidMutex.Lock()
	defer lidMutex.Unlock()
	
	count := 0
	for phone, info := range data.Bots {
		lidCache[phone] = info.LID
		count++
	}
	fmt.Printf("📂 Loaded %d LIDs from disk.\n", count)
}

func readJSON() (LIDStorage, error) {
	var data LIDStorage
	file, err := os.ReadFile(LIDDataFile)
	if err != nil { return data, err }
	
	err = json.Unmarshal(file, &data)
	return data, err
}

func saveJSON(data LIDStorage) {
	// فولڈر یقینی بنائیں
	dir := filepath.Dir(LIDDataFile)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, 0755)
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err == nil {
		os.WriteFile(LIDDataFile, bytes, 0644)
	}
}

// Helper (اگر commands.go میں نہیں ہے تو)
// func getCleanID(s string) string {
// 	if strings.Contains(s, ":") {
// 		return strings.Split(s, ":")[0]
// 	}
// 	return strings.Split(s, "@")[0]
// }
