package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
)

// ==========================================
// ⚙️ PREFIX & MODE SETTINGS
// ==========================================

// 🔡 SET PREFIX
func HandleSetPrefix(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		ReplyMessage(client, v, "⚠️ Usage: .setprefix <symbol>\nExample: .setprefix !")
		return
	}

	newPrefix := args[0]
	botID := getCleanID(client.Store.ID.User)

	// 🔒 LOCK & UPDATE
	sm.mu.Lock()
	if sm.Settings[botID] == nil {
		sm.Settings[botID] = &BotSettings{Mode: "public"}
	}
	sm.Settings[botID].Prefix = newPrefix
	sm.mu.Unlock()

	// 💾 SAVE TO DISK
	saveSettings()

	ReplyMessage(client, v, fmt.Sprintf("✅ Prefix updated to: [ %s ]", newPrefix))
}

// 🛡️ SET MODE (Public / Admin / Private)
func HandleMode(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		ReplyMessage(client, v, "⚠️ Usage: .mode public | admin | private")
		return
	}

	mode := strings.ToLower(args[0])
	if mode != "public" && mode != "admin" && mode != "private" {
		ReplyMessage(client, v, "❌ Invalid Mode! Use: public, admin, or private.")
		return
	}

	botID := getCleanID(client.Store.ID.User)

	// 🔒 LOCK & UPDATE
	sm.mu.Lock()
	if sm.Settings[botID] == nil {
		sm.Settings[botID] = &BotSettings{Prefix: "."}
	}
	sm.Settings[botID].Mode = mode
	sm.mu.Unlock()

	// 💾 SAVE
	saveSettings()

	ReplyMessage(client, v, fmt.Sprintf("🛡️ Bot Mode switched to: *%s*", strings.ToUpper(mode)))
}

// ==========================================
// 🔄 MASTER TOGGLE FUNCTION (AUTO READ/REACT etc)
// ==========================================

func HandleToggle(client *whatsmeow.Client, v *events.Message, command string) {
	botID := getCleanID(client.Store.ID.User)

	sm.mu.Lock()
	// Ensure settings exist
	if sm.Settings[botID] == nil {
		sm.Settings[botID] = &BotSettings{Prefix: ".", Mode: "public"}
	}
	
	settings := sm.Settings[botID]
	var newVal bool
	var featureName string

	// 🔀 SWITCH LOGIC
	switch command {
	case "alwaysonline":
		settings.AlwaysOnline = !settings.AlwaysOnline
		newVal = settings.AlwaysOnline
		featureName = "Always Online"
		
		// اگر ON کیا ہے تو ابھی Presense بھیج دیں
		if newVal {
			go client.SendPresence(types.PresenceAvailable)
		}

	case "autoread":
		settings.AutoRead = !settings.AutoRead
		newVal = settings.AutoRead
		featureName = "Auto Read (Blue Ticks)"

	case "autoreact":
		settings.AutoReact = !settings.AutoReact
		newVal = settings.AutoReact
		featureName = "Auto React (Personal)"

	case "autostatus":
		settings.AutoStatus = !settings.AutoStatus
		newVal = settings.AutoStatus
		featureName = "Auto View Status"

	case "statusreact":
		settings.StatusReact = !settings.StatusReact
		newVal = settings.StatusReact
		featureName = "Auto Status Like"
	
	case "welcomemsg":
		settings.WelcomeMsg = !settings.WelcomeMsg
		newVal = settings.WelcomeMsg
		featureName = "Welcome Message"
	}
	
	sm.mu.Unlock()
	
	// 💾 SAVE IMMEDIATELY
	saveSettings()

	status := "🔴 OFF"
	if newVal {
		status = "🟢 ON"
	}

	ReplyMessage(client, v, fmt.Sprintf("⚙️ *%s* is now %s", featureName, status))
}

// ==========================================
// 📊 SYSTEM STATS & BOTS
// ==========================================

// 📈 SYSTEM STATS
func HandleStats(client *whatsmeow.Client, v *events.Message) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(StartTime).Round(time.Second)
	activeSessions := 0
	
	sm.mu.RLock()
	activeSessions = len(sm.Clients)
	sm.mu.RUnlock()

	msg := fmt.Sprintf(`
╔══════════════════╗
║ 📊 𝗦𝗬𝗦𝗧𝗘𝗠 𝗦𝗧𝗔𝗧𝗨𝗦
╠══════════════════╣
║ ⏳ 𝗨𝗽𝘁𝗶𝗺𝗲: %s
║ 🤖 𝗔𝗰𝘁𝗶𝘃𝗲 𝗕𝗼𝘁𝘀: %d
║ 💾 𝗥𝗔𝗠 𝗨𝘀𝗮𝗴𝗲: %v MB
║ ⚙️ 𝗚𝗼𝗿𝗼𝘂𝘁𝗶𝗻𝗲𝘀: %d
║ 🛡️ 𝗢𝗦: %s
╚══════════════════╝
`, uptime, activeSessions, m.Alloc/1024/1024, runtime.NumGoroutine(), runtime.GOOS)

	ReplyMessage(client, v, msg)
}

// 🤖 LIST ACTIVE BOTS
func HandleListBots(client *whatsmeow.Client, v *events.Message) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if len(sm.Clients) == 0 {
		ReplyMessage(client, v, "❌ No active bots found.")
		return
	}

	msg := "🤖 *ACTIVE SESSIONS LIST*\n\n"
	i := 1
	for botID := range sm.Clients {
		msg += fmt.Sprintf("%d. %s\n", i, botID)
		i++
	}

	ReplyMessage(client, v, msg)
}

// ==========================================
// 🗑️ SESSION MANAGEMENT
// ==========================================

// 💀 DELETE SESSION (Log out specific number)
func HandleDeleteSession(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		ReplyMessage(client, v, "⚠️ Enter a number to delete session.\nExample: .sd 923001234567")
		return
	}

	// نمبر سے اسپیس ہٹائیں
	targetNum := strings.ReplaceAll(strings.Join(args, ""), " ", "")
	targetNum = strings.ReplaceAll(targetNum, "+", "")
	cleanID := getCleanID(targetNum)

	sm.mu.Lock()
	targetClient, exists := sm.Clients[cleanID]
	
	if exists {
		// 1. میموری سے کنکشن کاٹیں
		targetClient.Disconnect()
		delete(sm.Clients, cleanID)
		delete(sm.Settings, cleanID) // سیٹنگز بھی اڑا دیں
	}
	sm.mu.Unlock()

	// 2. ڈیٹا بیس سے ڈیلیٹ کریں
	devices, err := container.GetAllDevices()
	if err == nil {
		for _, d := range devices {
			if getCleanID(d.ID.User) == cleanID {
				d.Delete() // Permanent DB Delete
			}
		}
	}
	
	// 3. Save Changes
	saveSettings()

	ReplyMessage(client, v, fmt.Sprintf("🗑️ Session deleted for: %s", cleanID))
}
