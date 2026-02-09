package main

import (
	"context"
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"google.golang.org/protobuf/proto"
)

// ==========================================
// 🛠️ HELPER
// ==========================================
func GetTarget(v *events.Message, args []string) (types.JID, bool) {
	if v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.ContextInfo != nil {
		ctx := v.Message.ExtendedTextMessage.ContextInfo
		if ctx.Participant != nil {
			jid, _ := types.ParseJID(*ctx.Participant)
			return jid, true
		}
	}
	if v.Message.ExtendedTextMessage != nil && v.Message.ExtendedTextMessage.ContextInfo != nil {
		mentions := v.Message.ExtendedTextMessage.ContextInfo.MentionedJID
		if len(mentions) > 0 {
			jid, _ := types.ParseJID(mentions[0])
			return jid, true
		}
	}
	if len(args) > 0 {
		joinedNum := strings.Join(args, "")
		cleanNum := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' { return r }
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
// 🛡️ ADMIN ACTIONS (Direct Action)
// ==========================================

func HandleKick(client *whatsmeow.Client, v *events.Message, args []string) {
	target, found := GetTarget(v, args)
	if !found { ReplyMessage(client, v, "❌ Select user."); return }
	_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, whatsmeow.ParticipantChangeRemove)
	if err != nil { ReplyMessage(client, v, "❌ Failed.") } else { ReplyMessage(client, v, "👢 Kicked!") }
}

func HandleAdd(client *whatsmeow.Client, v *events.Message, args []string) {
	target, found := GetTarget(v, args)
	if !found { ReplyMessage(client, v, "❌ Provide number."); return }
	_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, whatsmeow.ParticipantChangeAdd)
	if err != nil { ReplyMessage(client, v, "❌ Failed.") } else { ReplyMessage(client, v, "➕ Added!") }
}

func HandlePromote(client *whatsmeow.Client, v *events.Message, args []string) {
	target, found := GetTarget(v, args)
	if !found { ReplyMessage(client, v, "❌ Select user."); return }
	_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, whatsmeow.ParticipantChangePromote)
	if err != nil { ReplyMessage(client, v, "❌ Failed.") } else { ReplyMessage(client, v, "⬆️ Promoted!") }
}

func HandleDemote(client *whatsmeow.Client, v *events.Message, args []string) {
	target, found := GetTarget(v, args)
	if !found { ReplyMessage(client, v, "❌ Select user."); return }
	_, err := client.UpdateGroupParticipants(context.Background(), v.Info.Chat, []types.JID{target}, whatsmeow.ParticipantChangeDemote)
	if err != nil { ReplyMessage(client, v, "❌ Failed.") } else { ReplyMessage(client, v, "⬇️ Demoted!") }
}

func HandleGroupSettings(client *whatsmeow.Client, v *events.Message, args []string) {
	if len(args) == 0 { ReplyMessage(client, v, "⚠️ Use .group open|close"); return }
	action := strings.ToLower(args[0])
	var err error
	if action == "close" { err = client.SetGroupAnnounce(context.Background(), v.Info.Chat, true) }
	if action == "open" { err = client.SetGroupAnnounce(context.Background(), v.Info.Chat, false) }
	if err != nil { ReplyMessage(client, v, "❌ Failed.") } else { ReplyMessage(client, v, "✅ Done.") }
}

func HandleDelete(client *whatsmeow.Client, v *events.Message) {
	if v.Message.ExtendedTextMessage == nil || v.Message.ExtendedTextMessage.ContextInfo == nil {
		ReplyMessage(client, v, "❌ Reply to delete.")
		return
	}
	ctx := v.Message.ExtendedTextMessage.ContextInfo
	targetID := ctx.StanzaID
	
	if targetID == nil { return }

	// ✅ FIX: Added context.Background() as first arg
	_, err := client.RevokeMessage(context.Background(), v.Info.Chat, types.MessageID(*targetID))
	
	if err != nil { ReplyMessage(client, v, "❌ Failed.") }
}

func HandleTagAll(client *whatsmeow.Client, v *events.Message, args []string) {
	groupInfo, err := client.GetGroupInfo(context.Background(), v.Info.Chat)
	if err != nil { ReplyMessage(client, v, "❌ Error."); return }
	text := "📣 *EVERYONE* " + strings.Join(args, " ")
	var mentions []string
	for _, p := range groupInfo.Participants { mentions = append(mentions, p.JID.String()) }
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{ MentionedJID: mentions },
		},
	})
}

func HandleHideTag(client *whatsmeow.Client, v *events.Message, args []string) {
	groupInfo, err := client.GetGroupInfo(context.Background(), v.Info.Chat)
	if err != nil { ReplyMessage(client, v, "❌ Error."); return }
	text := strings.Join(args, " ")
	if text == "" { text = "🔔" }
	var mentions []string
	for _, p := range groupInfo.Participants { mentions = append(mentions, p.JID.String()) }
	client.SendMessage(context.Background(), v.Info.Chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{ MentionedJID: mentions },
		},
	})
}
