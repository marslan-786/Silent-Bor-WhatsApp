package main

import (
	"context"
	"fmt"
	"strings"
	"os"
	"time"
	"sync"
    "strconv"
    
    "go.mau.fi/whatsmeow"
	"github.com/showwin/speedtest-go/speedtest"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

var RestrictedGroups = map[string]bool{
    "120363365896020486@g.us": true,
    "120363405060081993@g.us": true, 
}

var replyChannels = make(map[string]chan string)
var replyMutex sync.RWMutex

var AuthorizedBots = map[string]bool{
    "923017552805": true,
    "923116573691": true,
}

// ════════════════════════════════════════════════════════════════
// 🔗 MAIN HANDLER HOOK (Fixes Missing Handler Issue)
// ════════════════════════════════════════════════════════════════

// ✅ یہ فنکشن main.go کو commands.go سے جوڑتا ہے
func EventHandler(client *whatsmeow.Client) func(interface{}) {
	return func(evt interface{}) {
		handler(client, evt)
	}
}

// ════════════════════════════════════════════════════════════════
// ⚙️ CORE HANDLER LOGIC
// ════════════════════════════════════════════════════════════════

func handler(botClient *whatsmeow.Client, evt interface{}) {
	defer func() {
		if r := recover(); r != nil {
			bot := "unknown"
			if botClient != nil && botClient.Store != nil && botClient.Store.ID != nil {
				bot = botClient.Store.ID.User
			}
			fmt.Printf("⚠️ [CRASH PREVENTED] Bot %s error: %v\n", bot, r)
		}
	}()

	if botClient == nil {
		return
	}

	// go ListenForFeatures(botClient, evt) // اگر فیچرز فائل موجود ہے تو ان کمنٹ کریں

	switch v := evt.(type) {

	case *events.Message:
		// پرانے میسجز اگنور کریں (1 منٹ سے زیادہ پرانے)
		if time.Since(v.Info.Timestamp) > 1*time.Minute {
			return
		}

		botID := "unknown"
		if botClient.Store != nil && botClient.Store.ID != nil {
			botID = getCleanID(botClient.Store.ID.User)
		}

		// ✅ Save Message to Mongo (Background)
		go func() {
			saveMessageToMongo(
				botClient,
				botID,
				v.Info.Chat.String(),
				v.Info.Sender,
				v.Message,
				v.Info.IsFromMe,
				uint64(v.Info.Timestamp.Unix()),
			)
		}()

		// 🛑 Status Check
		if v.Info.Chat.String() == "status@broadcast" {
			return
		}

		// Process Commands
		go processMessage(botClient, v)

	case *events.Connected:
		if botClient.Store != nil && botClient.Store.ID != nil {
			fmt.Printf("🟢 [ONLINE] Bot %s connected!\n", botClient.Store.ID.User)
		}
	}
}

// ⚡ PERMISSION CHECK
func canExecute(client *whatsmeow.Client, v *events.Message, cmd string) bool {
	// 1. Owner Check
	if isOwner(client, v.Info.Sender) { return true }
	
	// 2. Private Chat Check
	if !v.Info.IsGroup { return true }

	// 3. Group Checks
	rawBotID := client.Store.ID.User
	botID := getCleanID(rawBotID)
	
	s := getGroupSettings(botID, v.Info.Chat.String())
	
	if s.Mode == "private" { return false }
	if s.Mode == "admin" { return isAdmin(client, v.Info.Chat, v.Info.Sender) }
	
	return true
}

// ⚡ MAIN MESSAGE PROCESSOR
func processMessage(client *whatsmeow.Client, v *events.Message) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("⚠️ Critical Panic: %v\n", r)
		}
	}()

	// 1. Extract Text
	bodyRaw := getText(v.Message)
	isAudio := v.Message.GetAudioMessage() != nil 

	if bodyRaw == "" && !isAudio {
		return
	}
	bodyClean := strings.TrimSpace(bodyRaw)

	// 2. Bot ID Info
	rawBotID := client.Store.ID.User
	botID := getCleanID(rawBotID)

	// 3. Variables
	chatID := v.Info.Chat.String()
	senderID := v.Info.Sender.ToNonAD().String()

	// 4. Prefix Check
	prefix := getPrefix(botID)
	isCommand := strings.HasPrefix(bodyClean, prefix)

	// 🔥 GLOBAL SETTINGS (RAM)
	dataMutex.RLock()
	doRead := data.AutoRead
	doReact := data.AutoReact
	dataMutex.RUnlock()

	// 🚀 BACKGROUND TASKS
	go func() {
		// A. Reply Interceptor (For Setup/Download Wizards)
		replyMutex.RLock()
		ch, waiting := replyChannels[senderID]
		replyMutex.RUnlock()

		if waiting {
			if bodyClean != "" {
				ch <- bodyClean
				replyMutex.Lock()
				delete(replyChannels, senderID)
				replyMutex.Unlock()
				return
			}
		}

		// B. Auto Read/React
		if doRead || doReact {
			if doRead {
				client.MarkRead(context.Background(), []types.MessageID{v.Info.ID}, v.Info.Timestamp, v.Info.Chat, v.Info.Sender)
			}
			if doReact {
				shouldReact := !v.Info.IsGroup
				if v.Info.IsGroup && (strings.Contains(bodyClean, "@"+botID) || isCommand) {
					shouldReact = true
				}
				if shouldReact {
					// react(client, v.Info.Chat, v.Info.ID, "❤️") // Optional
				}
			}
		}

		// C. Command Handling
		if !isCommand {
			return
		}

		msgWithoutPrefix := strings.TrimPrefix(bodyClean, prefix)
		words := strings.Fields(msgWithoutPrefix)
		if len(words) == 0 { return }

		cmd := strings.ToLower(words[0])
		var args []string
		if len(words) > 1 { args = words[1:] }
		fullArgs := strings.TrimSpace(strings.Join(args, " "))
		
		if !canExecute(client, v, cmd) { return }

		fmt.Printf("🚀 [EXEC] Bot:%s | CMD:%s\n", botID, cmd)

		// 🔥 COMMAND SWITCH 🔥
		switch cmd {

		// ✅ MENU COMMAND (ADDED HERE)
		case "menu", "help", "list":
			react(client, v.Info.Chat, v.Info.ID, "📂")
			sendMenu(client, v)

		case "ping":
			react(client, v.Info.Chat, v.Info.ID, "⚡")
			sendPing(client, v)
		
		case "id":
			react(client, v.Info.Chat, v.Info.ID, "🆔")
			sendID(client, v)

		case "owner":
			react(client, v.Info.Chat, v.Info.ID, "👑")
			sendOwner(client, v)
		
		case "listbots":
			react(client, v.Info.Chat, v.Info.ID, "🤖")
			sendBotsList(client, v)

		// ⚙️ SETTINGS
		case "setprefix":
			if !isOwner(client, v.Info.Sender) { return }
			if fullArgs == "" {
				replyMessage(client, v, "⚠️ Usage: .setprefix !")
				return
			}
			updatePrefixDB(botID, fullArgs)
			replyMessage(client, v, fmt.Sprintf("✅ Prefix updated to [%s]", fullArgs))

		case "mode":
			if !isOwner(client, v.Info.Sender) { return }
			handleMode(client, v, args)

		case "alwaysonline":
			if !isOwner(client, v.Info.Sender) { return }
			toggleAlwaysOnline(client, v)

		// 🛡️ ADMIN / GROUP
		case "kick":
			handleKick(client, v, args)
		case "add":
			handleAdd(client, v, args)
		case "tagall":
			handleTagAll(client, v, args)
		case "hidetag":
			handleHideTag(client, v, args)
		case "group":
			handleGroup(client, v, args)
		case "del", "delete":
			handleDelete(client, v)

		// 🛠️ TOOLS
		case "tr", "translate":
			handleTranslate(client, v, args)
		case "sticker", "s":
			handleToSticker(client, v)
		case "toimg":
			handleToImg(client, v)
		case "tourl":
			handleToURL(client, v)

		// 📥 DOWNLOADERS
		case "yt", "youtube":
			if fullArgs == "" {
				replyMessage(client, v, "⚠️ Send Link")
				return
			}
			handleYTDownloadMenu(client, v, fullArgs)
		
		case "tt", "tiktok":
			handleTikTok(client, v, fullArgs)
		case "fb", "facebook":
			handleFacebook(client, v, fullArgs)
		case "ig", "insta":
			handleInstagram(client, v, fullArgs)

		// 🔐 PRIVATE / OTP
		case "nset":
			HandleNSet(client, v, args)
		case "num":
			HandleGetNumber(client, v, args)
		case "code":
			HandleGetOTP(client, v, args)
		case "sd":
			handleSessionDelete(client, v, args)
		}
	}()
}

// ════════════════════════════════════════════════════════════════
// 🎨 MENU SENDER
// ════════════════════════════════════════════════════════════════

func sendMenu(client *whatsmeow.Client, v *events.Message) {
	// 📢 چینل کی سیٹنگز
	newsletterID := "120363424476167116@newsletter"
	newsletterName := "Silent Hackers Official"

	uptimeStr := getFormattedUptime()
	rawBotID := client.Store.ID.User
	botID := getCleanID(rawBotID)
	p := getPrefix(botID)
	
	s := getGroupSettings(botID, v.Info.Chat.String())
	currentMode := strings.ToUpper(s.Mode)
	if !v.Info.IsGroup { currentMode = "PRIVATE" }

	// مینیو ڈیزائن
	menu := fmt.Sprintf(`
      ｡ﾟﾟ･｡･ﾟﾟ｡
      ﾟ。    %s
      　ﾟ･｡･ﾟ
  
 👑 𝐎𝐰𝐧𝐞𝐫 : %s
 🛡️ 𝐌𝐨𝐝𝐞 : %s
 ⏳ 𝐔𝐩𝐭𝐢𝐦𝐞 : %s

   ⋆ 🎀 ⋆ ──── ⋆ 🎀 ⋆

 ╭── 🍭 𝐃𝐨𝐰𝐧𝐥𝐨𝐚𝐝𝐬 🍭 ──╮
 │ ❥ *%sdl* - Direct File/Link
 │ ❥ *%syt* - YouTube Video
 │ ❥ *%stt* - TikTok (No WM)
 │ ❥ *%sfb* - Facebook
 │ ❥ *%sig* - Instagram
 ╰───────────────╯

 ╭── ✨ 𝐌𝐚𝐠𝐢𝐜 𝐓𝐨𝐨𝐥𝐬 ✨ ──╮
 │ ❥ *%sai* - Gemini Chat
 │ ❥ *%str* - Translate
 │ ❥ *%sremini* - Enhance
 │ ❥ *%sremovebg* - Remove BG
 ╰───────────────╯

 ╭── 🎨 𝐄𝐝𝐢𝐭𝐢𝐧𝐠 ──╮
 │ ❥ *%ssticker* - Sticker
 │ ❥ *%stoimg* - To Image
 │ ❥ *%stourl* - To URL
 ╰───────────────╯

 ╭── 🛡️ 𝐆𝐫𝐨𝐮𝐩 ──╮
 │ ❥ *%skick* - Kick
 │ ❥ *%sadd* - Add
 │ ❥ *%stagall* - Tag All
 │ ❥ *%shidetag* - Hide Tag
 │ ❥ *%sgroup* - Open/Close
 ╰───────────────╯

 ╭── 👑 𝐎𝐰𝐧𝐞𝐫 ──╮
 │ ❥ *%ssetprefix* - Prefix
 │ ❥ *%salwaysonline* - Always On
 │ ❥ *%slistbots* - List Bots
 │ ❥ *%ssd* - Session Del
 │ ❥ *%snum* - Get Number
 ╰───────────────╯

      💖 𝙎𝙞𝙡𝙚𝙣𝙩 𝙃𝙖𝙘𝙠𝙚𝙧𝙨 💖
`,
		BOT_NAME, OWNER_NAME, currentMode, uptimeStr,
		p, p, p, p, p, // Downloads
		p, p, p, p,    // AI
		p, p, p,       // Editing
		p, p, p, p, p, // Group
		p, p, p, p, p, // Owner
	)

	// Context for Reply
	replyContext := &waProto.ContextInfo{
		StanzaID:      proto.String(v.Info.ID),
		Participant:   proto.String(v.Info.Sender.String()),
		QuotedMessage: v.Message,
		IsForwarded:   proto.Bool(true),
		ForwardedNewsletterMessageInfo: &waProto.ForwardedNewsletterMessageInfo{
			NewsletterJID:   proto.String(newsletterID),
			NewsletterName:  proto.String(newsletterName),
			ServerMessageID: proto.Int32(100),
		},
	}

	// 1. Try Cached Image
	if cachedMenuImage != nil {
		imgMsg := *cachedMenuImage 
		imgMsg.Caption = proto.String(menu)
		imgMsg.ContextInfo = replyContext 
		client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{ImageMessage: &imgMsg})
		return
	}

	// 2. Upload Image
	imgData, err := os.ReadFile("pic.png")
	if err == nil {
		uploadResp, err := client.Upload(context.Background(), imgData, whatsmeow.MediaImage)
		if err == nil {
			cachedMenuImage = &waProto.ImageMessage{
				URL:           proto.String(uploadResp.URL),
				DirectPath:    proto.String(uploadResp.DirectPath),
				MediaKey:      uploadResp.MediaKey,
				Mimetype:      proto.String("image/png"),
				FileEncSHA256: uploadResp.FileEncSHA256,
				FileSHA256:    uploadResp.FileSHA256,
				FileLength:    proto.Uint64(uint64(len(imgData))),
			}
			imgMsg := *cachedMenuImage
			imgMsg.Caption = proto.String(menu)
			imgMsg.ContextInfo = replyContext
			client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{ImageMessage: &imgMsg})
			return
		}
	}

	// 3. Fallback Text
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(menu),
			ContextInfo: replyContext,
		},
	})
}

// ════════════════════════════════════════════════════════════════
// 🔧 UTILS
// ════════════════════════════════════════════════════════════════

func getPrefix(botID string) string {
	prefixMutex.RLock()
	p, exists := botPrefixes[botID]
	prefixMutex.RUnlock()
	if exists { return p }
	// Redis Fallback
	if rdb != nil {
		val, err := rdb.Get(context.Background(), "prefix:"+botID).Result()
		if err == nil && val != "" {
			prefixMutex.Lock()
			botPrefixes[botID] = val
			prefixMutex.Unlock()
			return val
		}
	}
	return "." 
}

func getCleanID(jidStr string) string {
	if jidStr == "" { return "unknown" }
	parts := strings.Split(jidStr, "@")
	if len(parts) == 0 { return "unknown" }
	userPart := parts[0]
	if strings.Contains(userPart, ":") {
		userPart = strings.Split(userPart, ":")[0]
	}
	return strings.TrimSpace(userPart)
}

func replyMessage(client *whatsmeow.Client, v *events.Message, text string) {
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				StanzaID:      proto.String(v.Info.ID),
				Participant:   proto.String(v.Info.Sender.String()),
				QuotedMessage: v.Message,
			},
		},
	})
}

func react(client *whatsmeow.Client, chat types.JID, msgID types.MessageID, emoji string) {
	go func() {
		client.SendMessage(context.Background(), chat, &waProto.Message{
			ReactionMessage: &waProto.ReactionMessage{
				Key: &waProto.MessageKey{
					RemoteJID: proto.String(chat.String()),
					ID:        proto.String(string(msgID)),
					FromMe:    proto.Bool(false),
				},
				Text:              proto.String(emoji),
				SenderTimestampMS: proto.Int64(time.Now().UnixMilli()),
			},
		})
	}()
}

func getText(m *waProto.Message) string {
	if m.Conversation != nil { return *m.Conversation }
	if m.ExtendedTextMessage != nil && m.ExtendedTextMessage.Text != nil { return *m.ExtendedTextMessage.Text }
	if m.ImageMessage != nil && m.ImageMessage.Caption != nil { return *m.ImageMessage.Caption }
	if m.VideoMessage != nil && m.VideoMessage.Caption != nil { return *m.VideoMessage.Caption }
	return ""
}
