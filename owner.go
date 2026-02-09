package main

import (
	"context" // ✅ Fix
	"fmt"
	"runtime"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"        // ✅ Fix
	"go.mau.fi/whatsmeow/types/events"
)

// ==========================================
// ⚙️ PREFIX & MODE
// ==========================================

func HandleSetPrefix(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		ReplyMessage(client, v, "⚠️ Usage: .setprefix !")
		return
	}
	botID := getCleanID(client.Store.ID.User)
	sm.mu.Lock()
	if sm.Settings[botID] == nil { sm.Settings[botID] = &BotSettings{Prefix: "."} }
	sm.Settings[botID].Prefix = args[0]
	sm.mu.Unlock()
	saveSettings()
	ReplyMessage(client, v, "✅ Prefix Set!")
}

func HandleMode(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 { ReplyMessage(client, v, "⚠️ .mode public|private"); return }
	mode := strings.ToLower(args[0])
	botID := getCleanID(client.Store.ID.User)
	sm.mu.Lock()
	if sm.Settings[botID] == nil { sm.Settings[botID] = &BotSettings{} }
	sm.Settings[botID].Mode = mode
	sm.mu.Unlock()
	saveSettings()
	ReplyMessage(client, v, "✅ Mode Set: "+mode)
}

// ==========================================
// 🔄 TOGGLES
// ==========================================

func HandleToggle(client *whatsmeow.Client, v *events.Message, command string) {
	botID := getCleanID(client.Store.ID.User)
	sm.mu.Lock()
	if sm.Settings[botID] == nil { sm.Settings[botID] = &BotSettings{} }
	settings := sm.Settings[botID]
	
	status := "OFF"
	switch command {
	case "alwaysonline":
		settings.AlwaysOnline = !settings.AlwaysOnline
		if settings.AlwaysOnline { 
			// ✅ FIX: Added Context
			client.SendPresence(context.Background(), types.PresenceAvailable) 
			status = "ON"
		}
	case "autoread":
		settings.AutoRead = !settings.AutoRead; if settings.AutoRead { status = "ON" }
	case "autoreact":
		settings.AutoReact = !settings.AutoReact; if settings.AutoReact { status = "ON" }
	case "autostatus":
		settings.AutoStatus = !settings.AutoStatus; if settings.AutoStatus { status = "ON" }
	case "statusreact":
		settings.StatusReact = !settings.StatusReact; if settings.StatusReact { status = "ON" }
	}
	sm.mu.Unlock()
	saveSettings()
	ReplyMessage(client, v, fmt.Sprintf("⚙️ %s: %s", command, status))
}

// ==========================================
// 📊 STATS
// ==========================================

func HandleStats(client *whatsmeow.Client, v *events.Message) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	uptime := time.Since(StartTime).Round(time.Second)
	sm.mu.RLock()
	active := len(sm.Clients)
	sm.mu.RUnlock()
	msg := fmt.Sprintf("📊 *SYSTEM STATS*\n\n⏳ Uptime: %s\n🤖 Active Bots: %d\n💾 RAM: %v MB", uptime, active, m.Alloc/1024/1024)
	ReplyMessage(client, v, msg)
}

func HandleListBots(client *whatsmeow.Client, v *events.Message) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	msg := "🤖 *ACTIVE SESSIONS*\n"
	for id := range sm.Clients { msg += "• " + id + "\n" }
	ReplyMessage(client, v, msg)
}

func HandleDeleteSession(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 { ReplyMessage(client, v, "⚠️ Provide number"); return }
	target := strings.ReplaceAll(args[0], "+", "")
	sm.mu.Lock()
	if c, ok := sm.Clients[target]; ok {
		c.Disconnect()
		delete(sm.Clients, target)
		delete(sm.Settings, target)
	}
	sm.mu.Unlock()
	
	// DB delete logic requires iterating devices in main.go, 
	// for simplicity we just remove from RAM and Save settings here.
	saveSettings()
	ReplyMessage(client, v, "🗑️ Session Deleted (RAM Only)")
}
