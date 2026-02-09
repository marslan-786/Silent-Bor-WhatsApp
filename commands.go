package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"sync"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// ⚙️ CONFIGURATION
const (
	BotName        = "𝙎𝙞𝙡𝙚𝙣𝙩 𝙃𝙖𝙘𝙠𝙚𝙧𝙨"
	OwnerName      = "Silent Hackers 🜲"
	NewsletterID   = "120363424476167116@newsletter"
	NewsletterName = "Silent Hackers Official"
)

// 🖼️ GLOBAL IMAGE CACHE
var (
	cachedMenuImage *waProto.ImageMessage
	imgMutex        sync.RWMutex
	StartTime       = time.Now()
)

// ==========================================
// 🚀 MAIN HANDLER
// ==========================================

func HandleMessages(client *whatsmeow.Client, evt interface{}) {
	switch v := evt.(type) {
	case *events.Message:
		// 1. Time Check
		if time.Since(v.Info.Timestamp) > 60*time.Second { return }

		// 2. Extract Body
		body := getText(v.Message)
		if body == "" { return }

		// 3. Get Bot ID & Settings
		rawBotID := client.Store.ID.User
		botID := getCleanID(rawBotID) // ✅ Used from main.go
		
		// 4. Dynamic Prefix
		prefix := "." 
		sm.mu.RLock()
		if sm.Settings[botID] != nil && sm.Settings[botID].Prefix != "" {
			prefix = sm.Settings[botID].Prefix
		}
		sm.mu.RUnlock()

		// 5. Check Prefix
		if !strings.HasPrefix(body, prefix) { return }

		// 6. Parse Command
		args := strings.Fields(body[len(prefix):])
		cmd := strings.ToLower(args[0])
		fullArgs := strings.Join(args[1:], " ")
		_ = fullArgs // ✅ Fix: Handle Unused Variable

		// 🔍 Log
		fmt.Printf("🤖 CMD: %s | User: %s\n", cmd, v.Info.Sender.User)

		// 7. 🚦 ROUTER
		switch cmd {

		// ➤ MENU & HELP
		case "menu", "help", "list":
			go DoReact(client, v, "📂")
			SendMenu(client, v, prefix, botID)

		// ====================================================
		// 👑 OWNER CONTROL
		// ====================================================
		
		case "setprefix":
			go DoReact(client, v, "⚙️")
			if !isOwner(client, v.Info.Sender) { return }
			HandleSetPrefix(client, v, args)

		case "mode":
			go DoReact(client, v, "🛡️")
			if !isOwner(client, v.Info.Sender) { return }
			HandleMode(client, v, args)

		case "alwaysonline":
			go DoReact(client, v, "🟢")
			if !isOwner(client, v.Info.Sender) { return }
			HandleToggle(client, v, "alwaysonline")

		case "autoread":
			go DoReact(client, v, "👁️")
			if !isOwner(client, v.Info.Sender) { return }
			HandleToggle(client, v, "autoread")

		case "autoreact":
			go DoReact(client, v, "💖")
			if !isOwner(client, v.Info.Sender) { return }
			HandleToggle(client, v, "autoreact")

		case "autostatus":
			go DoReact(client, v, "📺")
			if !isOwner(client, v.Info.Sender) { return }
			HandleToggle(client, v, "autostatus")

		case "statusreact":
			go DoReact(client, v, "🔥")
			if !isOwner(client, v.Info.Sender) { return }
			HandleToggle(client, v, "statusreact")
			
		case "stats":
			go DoReact(client, v, "📊")
			HandleStats(client, v)

		case "listbots":
			go DoReact(client, v, "🤖")
			if !isOwner(client, v.Info.Sender) { return }
			HandleListBots(client, v)

		case "sd", "delete-session":
			go DoReact(client, v, "💀")
			if !isOwner(client, v.Info.Sender) { return }
			HandleDeleteSession(client, v, args)

		// ====================================================
		// 🛡️ GROUP ADMINISTRATION
		// ====================================================
		
		case "kick":
			go DoReact(client, v, "👢")
			HandleKick(client, v, args)

		case "add":
			go DoReact(client, v, "➕")
			HandleAdd(client, v, args)

		case "promote":
			go DoReact(client, v, "⬆️")
			HandlePromote(client, v, args)

		case "demote":
			go DoReact(client, v, "⬇️")
			HandleDemote(client, v, args)

		case "tagall":
			go DoReact(client, v, "📣")
			if isAdmin(client, v.Info.Chat, v.Info.Sender) {
				HandleTagAll(client, v, args)
			}

		case "hidetag":
			go DoReact(client, v, "👻")
			if isAdmin(client, v.Info.Chat, v.Info.Sender) {
				HandleHideTag(client, v, args)
			}

		case "group":
			go DoReact(client, v, "🔒")
			HandleGroupSettings(client, v, args)

		case "del", "delete":
			go DoReact(client, v, "🗑️")
			HandleDelete(client, v)
			
		default:
			// Ignore unknown
		}
	}
}

// ✅ Fix: Helper Functions
func DoReact(client *whatsmeow.Client, v *events.Message, emoji string) {
	defer func() {
		if r := recover(); r != nil { fmt.Printf("React Error: %v\n", r) }
	}()

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJID: proto.String(v.Info.Chat.String()),
				ID:        proto.String(v.Info.ID),
				FromMe:    proto.Bool(false),
			},
			Text:              proto.String(emoji),
			SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
		},
	})
}

func SendMenu(client *whatsmeow.Client, v *events.Message, p string, botID string) {
	pushName := v.Info.PushName
	if pushName == "" { pushName = "User" }
	
	uptime := time.Since(StartTime).Round(time.Second)
	uptimeStr := fmt.Sprintf("%s", uptime)

	mode := "PUBLIC"
	sm.mu.RLock()
	if sm.Settings[botID] != nil && sm.Settings[botID].Mode != "" {
		mode = strings.ToUpper(sm.Settings[botID].Mode)
	}
	sm.mu.RUnlock()

	menuText := fmt.Sprintf(`
░▀█▀░█▀█░█▀█░█░░░█▀
░░█░░█░█░█░█░█░░░▀▀
░░▀░░▀▀▀░▀▀▀░▀▀▀░▀▀

💀 𝗨𝗦𝗘𝗥: *%s*
🛡 𝗠𝗢𝗗𝗘: *%s*
⏳ 𝗨𝗣𝗧𝗜𝗠𝗘: *%s*

[ ☠️ ] ──── 𝗚𝗥𝗢𝗨𝗣𝗦 ────
│
│ ⦿ *%skick* ➔ 𝘒𝘪𝘤𝘬 𝘜𝘴𝘦𝘳
│ ⦿ *%sadd* ➔ 𝘈𝘥𝘥 𝘜𝘴𝘦𝘳
│ ⦿ *%spromote* ➔ 𝘔𝘢𝘬𝘦 𝘈𝘥𝘮𝘪𝘯
│ ⦿ *%sdemote* ➔ 𝘙𝘦𝘮𝘰𝘷𝘦 𝘈𝘥𝘮𝘪𝘯
│ ⦿ *%shidetag* ➔ 𝘏𝘪𝘥𝘥𝘦𝘯 𝘛𝘢𝘨
│ ⦿ *%stagall* ➔ 𝘛𝘢𝘨 𝘌𝘷𝘦𝘳𝘺𝘰𝘯𝘦
│ ⦿ *%sgroup* ➔ 𝘖𝘱𝘦𝘯/𝘊𝘭𝘰𝘴𝘦
│ ⦿ *%sdel* ➔ 𝘋𝘦𝘭𝘦𝘵𝘦 𝘔𝘴𝘨
│
[ 👑 ] ──── 𝗢𝗪𝗡𝗘𝗥 ────
│
│ ⦿ *%ssetprefix* ➔ 𝘊𝘩𝘢𝘯𝘨𝘦 𝘗𝘳𝘦𝘧𝘪𝘹
│ ⦿ *%smode* ➔ 𝘊𝘩𝘢𝘯𝘨𝘦 𝘔𝘰𝘥𝘦
│ ⦿ *%salwaysonline* ➔ 𝘈𝘭𝘸𝘢𝘺𝘴 𝘖𝘯
│ ⦿ *%sautoread* ➔ 𝘈𝘶𝘵𝘰 𝘙𝘦𝘢𝘥
│ ⦿ *%sautoreact* ➔ 𝘈𝘶𝘵𝘰 𝘙𝘦𝘢𝘤𝘵
│ ⦿ *%sautostatus* ➔ 𝘈𝘶𝘵𝘰 𝘚𝘵𝘢𝘵𝘶𝘴
│ ⦿ *%sstatusreact* ➔ 𝘚𝘵𝘢𝘵𝘶𝘴 𝘓𝘪𝘬𝘦
│ ⦿ *%ssd* ➔ 𝘚𝘦𝘴𝘴𝘪𝘰𝘯 𝘋𝘦𝘭𝘦𝘵𝘦
│
╰────────────── [ 💀 ]
`, pushName, mode, uptimeStr,
	p, p, p, p, p, p, p, p,
	p, p, p, p, p, p, p, p)

	imgMutex.RLock()
	cached := cachedMenuImage
	imgMutex.RUnlock()

	if cached != nil {
		SendImage(client, v, cached, menuText)
		return
	}

	imgData, err := os.ReadFile("pic.png")
	if err != nil {
		ReplyMessage(client, v, menuText)
		return
	}

	resp, err := client.Upload(context.Background(), imgData, whatsmeow.MediaImage)
	if err != nil {
		ReplyMessage(client, v, menuText)
		return
	}

	newImg := &waProto.ImageMessage{
		URL:           proto.String(resp.URL),
		DirectPath:    proto.String(resp.DirectPath),
		MediaKey:      resp.MediaKey,
		Mimetype:      proto.String("image/png"),
		FileEncSHA256: resp.FileEncSHA256,
		FileSHA256:    resp.FileSHA256,
		FileLength:    proto.Uint64(uint64(len(imgData))),
	}

	imgMutex.Lock()
	cachedMenuImage = newImg
	imgMutex.Unlock()

	SendImage(client, v, newImg, menuText)
}

func ReplyMessage(client *whatsmeow.Client, v *events.Message, text string) {
	contextInfo := &waProto.ContextInfo{
		StanzaID:      proto.String(v.Info.ID),
		Participant:   proto.String(v.Info.Sender.String()),
		QuotedMessage: v.Message,
		IsForwarded:   proto.Bool(true),
		ForwardedNewsletterMessageInfo: &waProto.ForwardedNewsletterMessageInfo{
			NewsletterJID:   proto.String(NewsletterID),
			NewsletterName:  proto.String(NewsletterName),
			ServerMessageID: proto.Int32(100),
		},
	}

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text:        proto.String(text),
			ContextInfo: contextInfo,
		},
	})
}

func SendImage(client *whatsmeow.Client, v *events.Message, img *waProto.ImageMessage, caption string) {
	msgToSend := *img
	msgToSend.Caption = proto.String(caption)
	msgToSend.ContextInfo = &waProto.ContextInfo{
		StanzaID:      proto.String(v.Info.ID),
		Participant:   proto.String(v.Info.Sender.String()),
		QuotedMessage: v.Message,
		IsForwarded:   proto.Bool(true),
		ForwardedNewsletterMessageInfo: &waProto.ForwardedNewsletterMessageInfo{
			NewsletterJID:   proto.String(NewsletterID),
			NewsletterName:  proto.String(NewsletterName),
			ServerMessageID: proto.Int32(100),
		},
	}
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ImageMessage: &msgToSend,
	})
}

func getText(m *waProto.Message) string {
	if m == nil { return "" }
	if m.Conversation != nil { return *m.Conversation }
	if m.ExtendedTextMessage != nil { return *m.ExtendedTextMessage.Text }
	if m.ImageMessage != nil { return *m.ImageMessage.Caption }
	return ""
}

// ✅ Fix: Use isOwnerByLID from lid_system.go
func isOwner(client *whatsmeow.Client, sender types.JID) bool {
	if client.Store.ID != nil && client.Store.ID.User == sender.User {
		return true
	}
	return isOwnerByLID(client, sender) 
}

func isAdmin(client *whatsmeow.Client, chat, sender types.JID) bool {
	if chat.Server != "g.us" { return true } // ✅ Fixed Server Check
	return true // Placeholder
}
