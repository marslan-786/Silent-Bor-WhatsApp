package main

import (
	"sync"
	"go.mau.fi/whatsmeow"
)

// ==========================================
// 📦 GLOBAL TYPES & STRUCTURES
// ==========================================

// 1. BotSettings: ہر بوٹ کی انفرادی سیٹنگز
type BotSettings struct {
	AutoRead     bool   `json:"auto_read"`
	AutoReact    bool   `json:"auto_react"`
	AutoStatus   bool   `json:"auto_status"`
	StatusReact  bool   `json:"status_react"`
	AlwaysOnline bool   `json:"always_online"`
	Prefix       string `json:"prefix"`
	WelcomeMsg   bool   `json:"welcome_msg"` // اگر ویلکم میسج فیچر چاہیے
}

// 2. SessionManager: تمام بوٹس اور ان کا ڈیٹا سنبھالنے والا
type SessionManager struct {
	// Active Clients (RAM میں موجود کنکشنز)
	Clients map[string]*whatsmeow.Client
	
	// Settings (RAM میں موجود سیٹنگز)
	Settings map[string]*BotSettings
	
	// Mutex (تاکہ ایک وقت میں دو پروسیس ڈیٹا خراب نہ کریں)
	mu sync.RWMutex
}

// 3. Web Socket Message: فرنٹ اینڈ کو ڈیٹا بھیجنے کے لیے
type WSMessage struct {
	Type       string      `json:"type"`
	ActiveBots int         `json:"active_bots,omitempty"`
	BotID      string      `json:"bot_id,omitempty"`
	Payload    interface{} `json:"payload,omitempty"`
}

// 4. Pair Request: فرنٹ اینڈ سے آنے والی پیئرنگ ریکویسٹ
type PairRequest struct {
	Number string `json:"number"`
}

// types.go میں یہ اپڈیٹ کر لیں اگر نہیں ہے:
type BotSettings struct {
    // ... باقی فیلڈز
    Mode string `json:"mode"` // public, admin, private
}
