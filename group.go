package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// ==========================================
// 🛠️ HELPER: SMART TARGET EXTRACTOR
// ==========================================
func GetTarget(v *events.Message, args []string) (types.JID, bool) {
	// 1. Check Reply
	if v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.ContextInfo != nil {
		ctx := v.Message.ExtendedTextMessage.ContextInfo
		if ctx.Participant != nil {
			jid, _ := types.ParseJID(*ctx.Participant)
			return jid, true
		}
	}

	// 2. Check Mention
	if v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.ContextInfo != nil {
		mentions := v.Message.ExtendedTextMessage.ContextInfo.MentionedJID
		if len(mentions) > 0 {
			jid, _ := types.ParseJID(mentions[0])
			return jid, true
		}
	}

	// 3. Check Number in Args (Fix Spaces)
	if len(args) > 0 {
		// تمام ارگومنٹس کو جوڑ کر اسپیس ختم کریں (e.g. "92 300 123" -> "92300123")
		joinedNum := strings.Join(args, "")
		cleanNum := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, joinedNum)

		if len(cleanNum) > 7 {
			jid, _ := types.ParseJID(cleanNum + "@s.whatsapp.net")
			return jid, true
		}
	}

	return types.EmptyJID, false
}

// ==========================================
// 🛡️ ADMIN COMMANDS (ACTION FIRST LOGIC)
// ==========================================

// 👢 KICK USER (Direct Action)
func HandleKick(client *whatsmeow.Client, v *events.Message, args []string) {
	target, found := GetTarget(v, args)
	if !found {
		ReplyMessage(client, v, "❌ Reply to user or provide number.")
		return
	}

	// ⚡ DIRECT ACTION: Try to kick immediately
	_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, whatsmeow.ParticipantChangeRemove)
	
	if err != nil {
		// اگر ایرر آیا تو اس کا مطلب یا تو ہم ایڈمن نہیں ہیں یا بوٹ کو پرمیشن نہیں
		ReplyMessage(client, v, "❌ Failed! I need Admin rights.")
	} else {
		ReplyMessage(client, v, "👢 Kicked!")
	}
}

// ➕ ADD USER (Direct Action)
func HandleAdd(client *whatsmeow.Client, v *events.Message, args []string) {
	target, found := GetTarget(v, args)
	if !found {
		ReplyMessage(client, v, "❌ Provide number to add.\nExample: .add 923001234567")
		return
	}

	// ⚡ DIRECT ACTION
	_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, whatsmeow.ParticipantChangeAdd)
	
	if err != nil {
		ReplyMessage(client, v, "❌ Failed! Check privacy settings or my admin rights.")
	} else {
		ReplyMessage(client, v, "➕ User Added!")
	}
}

// ⬆️ PROMOTE USER (Direct Action)
func HandlePromote(client *whatsmeow.Client, v *events.Message, args []string) {
	target, found := GetTarget(v, args)
	if !found {
		ReplyMessage(client, v, "❌ Select user to Promote.")
		return
	}

	// ⚡ DIRECT ACTION
	_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, whatsmeow.ParticipantChangePromote)
	
	if err != nil {
		ReplyMessage(client, v, "❌ Failed! Am I Admin?")
	} else {
		ReplyMessage(client, v, "⬆️ Promoted to Admin!")
	}
}

// ⬇️ DEMOTE USER (Direct Action)
func HandleDemote(client *whatsmeow.Client, v *events.Message, args []string) {
	target, found := GetTarget(v, args)
	if !found {
		ReplyMessage(client, v, "❌ Select user to Demote.")
		return
	}

	// ⚡ DIRECT ACTION
	_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, whatsmeow.ParticipantChangeDemote)
	
	if err != nil {
		ReplyMessage(client, v, "❌ Failed! Am I Admin?")
	} else {
		ReplyMessage(client, v, "⬇️ Demoted from Admin!")
	}
}

// ==========================================
// ⚙️ GROUP SETTINGS (OPEN / CLOSE)
// ==========================================

func HandleGroupSettings(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 {
		ReplyMessage(client, v, "⚠️ Usage: .group open | close")
		return
	}

	action := strings.ToLower(args[0])
	var err error
	
	// ⚡ DIRECT ACTION
	if action == "close" || action == "off" {
		// Announce = true (Only Admins can send messages)
		err = client.SetGroupAnnounce(context.Background(), v.Info.Chat, true)
		if err == nil { 
			ReplyMessage(client, v, "🔒 Group Closed!") 
		}
	} else if action == "open" || action == "on" {
		// Announce = false (Everyone can send messages)
		err = client.SetGroupAnnounce(context.Background(), v.Info.Chat, false)
		if err == nil { 
			ReplyMessage(client, v, "🔓 Group Opened!") 
		}
	} else {
		ReplyMessage(client, v, "⚠️ Invalid Option. Use 'open' or 'close'.")
		return
	}

	if err != nil {
		ReplyMessage(client, v, "❌ Failed! I need Admin rights.")
	}
}

// ==========================================
// 🗑️ DELETE MESSAGE (Direct Action)
// ==========================================

func HandleDelete(client *whatsmeow.Client, v *events.Message) {
	// چیک کریں کہ رپلائی ہے یا نہیں
	if v.Message.ExtendedTextMessage == nil || v.Message.ExtendedTextMessage.ContextInfo == nil {
		ReplyMessage(client, v, "❌ Reply to a message to delete it.")
		return
	}

	ctx := v.Message.ExtendedTextMessage.ContextInfo
	targetID := ctx.StanzaID
	targetSender := ctx.Participant
	// اگر یوزر نے بوٹ کے اپنے میسج پر رپلائی کیا ہے تو 'Participant' nil ہو سکتا ہے (پرائیویٹ چیٹ میں)،
	// لیکن گروپ میں Participant ہوتا ہے۔ احتیاطاً چیک:
	if targetSender == nil && v.Info.IsGroup {
		// کچھ کیسز میں خود کا میسج ہو سکتا ہے
		// ہم بس ID استعمال کریں گے
	}

	if targetID == nil {
		return
	}

	// ⚡ DIRECT ACTION: Revoke Message
	// Revoke کرنے کے لیے Sender کی JID چاہیے ہوتی ہے (چاہے وہ کوئی بھی ہو)
	// اگر ہم ایڈمن ہیں تو کسی کا بھی میسج ڈیلیٹ کر سکتے ہیں
	
	var targetJID types.JID
	if targetSender != nil {
		targetJID, _ = types.ParseJID(*targetSender)
	} else {
		// اگر participant نہیں ملا تو شاید یہ بوٹ کا اپنا میسج ہے
		targetJID = client.Store.ID.ToNonAD() 
	}

	err := client.RevokeMessage(context.Background(), v.Info.Chat, types.MessageID(*targetID), targetJID)
	
	if err != nil {
		// یہ تب فیل ہوگا جب ہم ایڈمن نہ ہوں یا میسج بہت پرانا ہو
		ReplyMessage(client, v, "❌ Failed to delete! (Need Admin or msg too old)")
	}
}

// ==========================================
// 📣 TAGGING COMMANDS
// ==========================================

func HandleTagAll(client *whatsmeow.Client, v *events.Message, args []string) {
	groupInfo, err := client.GetGroupInfo(context.Background(), v.Info.Chat)
	if err != nil {
		ReplyMessage(client, v, "❌ Failed to fetch group info.")
		return
	}

	text := "📣 *EVERYONE MENTIONED*\n\n"
	if len(args) > 0 {
		text += "📝 Note: " + strings.Join(args, " ") + "\n\n"
	}

	var mentions []string
	for _, p := range groupInfo.Participants {
		text += fmt.Sprintf("➤ @%s\n", p.JID.User)
		mentions = append(mentions, p.JID.String())
	}

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentions,
				IsForwarded:  proto.Bool(true),
			},
		},
	})
}

func HandleHideTag(client *whatsmeow.Client, v *events.Message, args []string) {
	groupInfo, err := client.GetGroupInfo(context.Background(), v.Info.Chat)
	if err != nil {
		ReplyMessage(client, v, "❌ Failed to fetch group info.")
		return
	}

	var mentions []string
	for _, p := range groupInfo.Participants {
		mentions = append(mentions, p.JID.String())
	}

	// میسج باڈی (اگر رپلائی ہے تو وہ، ورنہ ٹیکسٹ)
	text := strings.Join(args, " ")
	if text == "" { text = "🔔 Hidetag!" }

	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentions, // سب کو ٹیگ کریں لیکن لسٹ نہ دکھائیں
			},
		},
	})
}
