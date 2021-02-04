package helpers

import (
	"encoding/json"
	"fmt"
	"go-tdlib/client"
	"log"
	"path/filepath"
	"strconv"
	"tgWatch/config"
	"time"
)

func initTdlib() {
	authorizer := client.ClientAuthorizer()
	go client.CliInteractor(authorizer)

	authorizer.TdlibParameters <- &client.TdlibParameters{
		UseTestDc:              false,
		DatabaseDirectory:      filepath.Join(".tdlib", "database"),
		FilesDirectory:         filepath.Join(".tdlib", "files"),
		UseFileDatabase:        true,
		UseChatInfoDatabase:    true,
		UseMessageDatabase:     true,
		UseSecretChats:         false,
		ApiId:                  config.Config.ApiId,
		ApiHash:                config.Config.ApiHash,
		SystemLanguageCode:     "en",
		DeviceModel:            "Linux",
		SystemVersion:          "1.0.0",
		ApplicationVersion:     "1.0.0",
		EnableStorageOptimizer: true,
		IgnoreFileNames:        false,
	}

	logVerbosity := client.WithLogVerbosity(&client.SetLogVerbosityLevelRequest{
		NewVerbosityLevel: 0,
	})

	var err error
	tdlibClient, err = client.NewClient(authorizer, logVerbosity)
	if err != nil {
		log.Fatalf("NewClient error: %s", err)
	}

	optionValue, err := tdlibClient.GetOption(&client.GetOptionRequest{
		Name: "version",
	})
	if err != nil {
		log.Fatalf("GetOption error: %s", err)
	}

	log.Printf("TDLib version: %s", optionValue.(*client.OptionValueString).Value)

	me, err := tdlibClient.GetMe()
	if err != nil {
		log.Fatalf("GetMe error: %s", err)
	}

	log.Printf("Me: %s %s [%s]", me.FirstName, me.LastName, me.Username)
}

func ListenUpdates()  {
	listener := tdlibClient.GetListener()
	defer listener.Close()

	for update := range listener.Updates {
		if update.GetClass() == client.ClassUpdate {
			t := update.GetType()
			switch t {
			case "updateUserFullInfo":
			case "updateChatActionBar":
			case "updateChatIsBlocked":
			case "updateChatPosition":
			case "updateChatFilters":

			case "updateOption":
			case "updateChatDraftMessage":
			case "updateUserStatus":
			case "updateChatReadInbox":
			case "updateChatReadOutbox":
			case "updateUnreadMessageCount":
			case "updateUnreadChatCount":
			case "updateChatLastMessage":
			case "updateUserChatAction":
			case "updateMessageInteractionInfo":
			case "updateChatReplyMarkup":
			case "updateChatPermissions":
			case "updateChatNotificationSettings":
			case "updateChatUnreadMentionCount":
			case "updateMessageMentionRead":
			case "updateConnectionState":
			case "updateMessageIsPinned":
			case "updateChatHasScheduledMessages":

			case "updateNewChat":
			case "updateHavePendingNotifications":
			case "updateSupergroupFullInfo":
			case "updateSupergroup":
			case "updateBasicGroup":
			case "updateBasicGroupFullInfo":
			case "updateChatPhoto":
			case "updateUser":
			case "updateChatTitle":
			case "updateDeleteMessages":
				break

			case "updateNewMessage":
				upd := update.(*client.UpdateNewMessage)
				senderChatId := GetChatIdBySender(upd.Message.Sender)
				if config.Config.IgnoreChatIds[strconv.FormatInt(upd.Message.ChatId, 10)] || config.Config.IgnoreAuthorIds[strconv.FormatInt(senderChatId, 10)] {

					break
				}
				mongoId := SaveUpdate(t, upd, upd.Message.Date)

				link := GetLink(tdlibClient, upd.Message.ChatId, upd.Message.Id)
				chatName := GetChatName(upd.Message.ChatId)
				intLink := fmt.Sprintf("http://%s/%d/%d", config.Config.WebListen, upd.Message.ChatId, upd.Message.Id)
				log.Printf("[%s] New Message from chat: %d, `%s`, %s, %s", mongoId, upd.Message.ChatId, chatName, link, intLink)

				break
			case "updateMessageEdited":
				upd := update.(*client.UpdateMessageEdited)
				if config.Config.IgnoreChatIds[strconv.FormatInt(upd.ChatId, 10)] {

					break
				}

				if upd.ReplyMarkup != nil {
					//log.Printf("SKIP EDITED msg! Chat: %d, msg %d, %s | %s", upd.ChatId, upd.MessageId, chatName, jsonMarshalStr(upd.ReplyMarkup))

					break
				}
				mongoId := SaveUpdate(t, upd, upd.EditDate)
				link := GetLink(tdlibClient, upd.ChatId, upd.MessageId)
				chatName := GetChatName(upd.ChatId)
				intLink := fmt.Sprintf("http://%s/%d/%d", config.Config.WebListen, upd.ChatId, upd.MessageId)
				log.Printf("[%s] EDITED msg! Chat: %d, msg %d, `%s`, %s, %s", mongoId, upd.ChatId, upd.MessageId, chatName, link, intLink)

				break
			case "updateMessageContent":
				upd := update.(*client.UpdateMessageContent)
				if config.Config.IgnoreChatIds[strconv.FormatInt(upd.ChatId, 10)] {

					break
				}
				if upd.NewContent.MessageContentType() == "messagePoll" {
					//dont save "poll" updates - that's just counters, users cannot update polls manually
					break
				}
				mongoId := SaveUpdate(t, upd, 0)

				link := GetLink(tdlibClient, upd.ChatId, upd.MessageId)
				chatName := GetChatName(upd.ChatId)
				log.Printf("[%s] EDITED content! Chat: %d, msg %d, %s, %s", mongoId, upd.ChatId, upd.MessageId, chatName, link)
				log.Printf("%s", GetContent(upd.NewContent))

				break
			default:
				log.Printf("%s : %#v", t, update)
			}
		}
	}
}

func GetChatIdBySender(sender client.MessageSender) int64 {
	senderChatId := int64(0)
	if sender.MessageSenderType() == "messageSenderChat" {
		senderChatId = sender.(*client.MessageSenderChat).ChatId
	} else if sender.MessageSenderType() == "messageSenderUser" {
		senderChatId = int64(sender.(*client.MessageSenderUser).UserId)
	}

	return senderChatId
}

func GetSenderName(sender client.MessageSender) string {
	if sender.MessageSenderType() == "messageSenderChat" {
		chatId := sender.(*client.MessageSenderChat).ChatId
		chatReq := &client.GetChatRequest{ChatId: chatId}
		chat, err := tdlibClient.GetChat(chatReq)
		if err != nil {
			log.Printf("Failed to request chat info by id %d: %s", chatId, err)

			return "unkown_chat";
		}
		return fmt.Sprintf("%s", chat.Title)
	} else if sender.MessageSenderType() == "messageSenderUser" {
		userId := sender.(*client.MessageSenderUser).UserId
		user, err := GetUser(userId)
		if err != nil {
			log.Printf("Failed to request user info by id %d: %s", userId, err)

			return "unkown_user"
		}
		name := ""
		if user.FirstName != "" {
			name = user.FirstName
		}
		if user.LastName != "" {
			name = fmt.Sprintf("%s %s", name, user.LastName)
		}
		if user.Username != "" {
			name = fmt.Sprintf("%s (@%s)", name, user.Username)
		}
		return name
	}
	log.Printf("Unknown sender chat type: %s", sender.MessageSenderType())

	return "unkown_chattype"
}

func GetLink(tdlibClient *client.Client, chatId int64, messageId int64) string {
	linkReq := &client.GetMessageLinkRequest{ChatId: chatId, MessageId: messageId}
	link, err := tdlibClient.GetMessageLink(linkReq)
	if err != nil {
		if err.Error() != "400 Public message links are available only for messages in supergroups and channel chats" {
			log.Printf("Failed to get msg link by chat id %d, msg id %d: %s", chatId, messageId, err)
		}

		return "no_link"
	}

	return link.Link
}

func GetChatName(chatId int64) string {
	fullChat, err := GetChat(chatId)
	if err != nil {
		log.Printf("Failed to get chat name by id %d: %s", chatId, err)

		return "no_title"
	}

	return fmt.Sprintf("%s", fullChat.Title)
}

func GetChat(chatId int64) (*client.Chat, error) {
	req := &client.GetChatRequest{ChatId: chatId}
	fullChat, err := tdlibClient.GetChat(req)

	return fullChat, err
}
func GetUser(userId int32) (*client.User, error) {
	userReq := &client.GetUserRequest{UserId: userId}

	return tdlibClient.GetUser(userReq)
}

func GetContent(content client.MessageContent) string {
	cType := content.MessageContentType()
	switch cType {
	case "messageText":
		msg := content.(*client.MessageText)

		return fmt.Sprintf("%s", msg.Text.Text)
	case "messagePhoto":
		msg := content.(*client.MessagePhoto)

		return fmt.Sprintf("Photo, %s", msg.Caption.Text)
	case "messageVideo":
		msg := content.(*client.MessageVideo)

		return fmt.Sprintf("Video, %s", msg.Caption.Text)
	case "messageAnimation":
		msg := content.(*client.MessageAnimation)

		return fmt.Sprintf("GIF, %s", msg.Caption.Text)
	case "messagePoll":
		msg := content.(*client.MessagePoll)

		return fmt.Sprintf("Poll, %s", msg.Poll.Question)
	default:

		return jsonMarshalStr(content)
	}
}

func jsonMarshalStr(j interface{}) string {
	m, err := json.Marshal(j)
	if err != nil {

		return "INVALID_JSON"
	}

	return string(m)
}


func getChatsList(tdlibClient *client.Client, ) {
	maxChatId := client.JsonInt64(int64((^uint64(0)) >> 1))
	offsetOrder := maxChatId
	log.Printf("Requesting chats with max id: %d", maxChatId)

	page := 0
	offsetChatId := int64(0)
	for {
		log.Printf("GetChats requesting page %d, offset %d", page, offsetChatId)
		chatsRequest := &client.GetChatsRequest{OffsetOrder: offsetOrder, Limit: 10, OffsetChatId: offsetChatId}
		chats, err := tdlibClient.GetChats(chatsRequest)
		if err != nil {
			log.Fatalf("[ERROR] GetChats: %s", err)
		}
		log.Printf("GetChats got page %d with %d chats", page, chats.TotalCount)
		for _, chatId := range chats.ChatIds {
			log.Printf("New ChatID %d", chatId)
			chatRequest := &client.GetChatRequest{ChatId: chatId}
			chat, err := tdlibClient.GetChat(chatRequest)
			if err != nil {
				log.Printf("[ERROR] GetChat id %d: %s", chatId, err)

				continue
			}
			log.Printf("Got chatID %d, position %d, title `%s`", chatId, chat.Positions[0].Order, chat.Title)
			offsetChatId = chat.Id
			offsetOrder = chat.Positions[0].Order
		}

		if len(chats.ChatIds) == 0 {
			log.Printf("Reached end of the list")

			break
		}
		time.Sleep(1 * time.Second)
		page++
		log.Println()
	}
}