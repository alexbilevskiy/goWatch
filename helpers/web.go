package helpers

import (
	"encoding/json"
	"fmt"
	"go-tdlib/client"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"tgWatch/config"
	"tgWatch/structs"
)

var verbose bool = false

func initWeb() {
	server := &http.Server{
		Addr:    config.Config.WebListen,
		Handler: HttpHandler{},
	}
	go server.ListenAndServe()
}

type HttpHandler struct{}
func (h HttpHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/" {
		req.URL.Path = "index.html"
	}
	path := "web/" + req.URL.Path
	stat, err := os.Stat(path);
	if err == nil && !stat.IsDir() {
		http.ServeFile(res, req, path)

		return
	}

	log.Printf("HTTP: %s", req.URL.Path)
	r := regexp.MustCompile(`^/([a-z]+?)($|/.+$)`)

	m := r.FindStringSubmatch(req.URL.Path)
	if m == nil {
		res.WriteHeader(404)
		res.Write([]byte("not found "+ req.URL.Path))

		return
	}
	data := []byte(fmt.Sprintf("Request URL: %s", req.RequestURI))

	action := m[1]
	req.ParseForm()
	if req.FormValue("a") == "1" {
		verbose = true
	} else {
		verbose = false
	}

	switch action {
	case "routes":

		res.Write([]byte(`{"routes":[{"/j":"journal"}]}`))

		return
	case "e":
		r := regexp.MustCompile(`^/e/(-?\d+)/(\d+)$`)
		m := r.FindStringSubmatch(req.URL.Path)
		if m == nil {
			data := []byte(fmt.Sprintf("Unknown path %s %s", action, req.URL.Path))
			res.Write(data)

			return
		}
		chatId, _ := strconv.ParseInt(m[1], 10, 64)
		messageId, _ := strconv.ParseInt(m[2], 10, 64)
		data = []byte(processTgEdit(chatId, messageId))
		break
	case "d":
		r := regexp.MustCompile(`^/d/(-?\d+)/([\d,]+)$`)
		m := r.FindStringSubmatch(req.URL.Path)
		if m == nil {
			data := []byte(fmt.Sprintf("Unknown path %s %s", action, req.URL.Path))
			res.Write(data)

			return
		}
		chatId, _ := strconv.ParseInt(m[1], 10, 64)
		messageIds := m[2]
		data = processTgDelete(chatId, ExplodeInt(messageIds))
		break
	case "j":
		limit := int64(50)
		if req.FormValue("limit") != "" {
			limit, _ = strconv.ParseInt(req.FormValue("limit"), 10, 64)
		}
		data = processTgJournal(limit)
		break
	case "c":
		r := regexp.MustCompile(`^/c/(-?\d+)$`)
		m := r.FindStringSubmatch(req.URL.Path)
		if m == nil {
			data := []byte(fmt.Sprintf("Unknown path %s %s", action, req.URL.Path))
			res.Write(data)

			return
		}
		chatId, _ := strconv.ParseInt(m[1], 10, 64)
		data = processTgChat(chatId)
		break
	default:
		res.WriteHeader(404)
		res.Write([]byte("not found " + req.URL.Path))

		return
	}

	res.WriteHeader(200)
	res.Write(data)
}

func processTgJournal(limit int64) []byte {
	fc := "<html><body>"

	updates, updateTypes, dates, err := FindRecentChanges(limit)
	if err != nil {

		fc = "Error: " + err.Error()

		return []byte(fc)
	}

	for i, rawJsonBytes := range updates {
		switch updateTypes[i] {
		case "updateNewMessage":
			upd, _ := client.UnmarshalUpdateNewMessage(rawJsonBytes)
			fc += fmt.Sprintf("[%s] New %s<br>", FormatTime(dates[i]), formatNewMessageLink(upd))
			break
		case "updateMessageEdited":
			upd, _ := client.UnmarshalUpdateMessageEdited(rawJsonBytes)
			fc += fmt.Sprintf("[%s] Edited message in chat \"%s\"<br>", FormatTime(dates[i]), GetChatName(upd.ChatId))
			break
		case "updateMessageContent":
			upd, _ := client.UnmarshalUpdateMessageContent(rawJsonBytes)
			fc += fmt.Sprintf("[%s] Updated %s<br>", FormatTime(dates[i]), formatUpdatedContentLink(upd))
			break
		case "updateDeleteMessages":
			upd, _ := client.UnmarshalUpdateDeleteMessages(rawJsonBytes)
			fc += fmt.Sprintf("[%s] Deleted %s<br>", FormatTime(dates[i]), formatDeletedMessagesLink(upd))
			break
		default:
			fc += fmt.Sprintf("[%s] Unknown update type \"%s\"<br>", FormatTime(dates[i]), updateTypes[i])
		}
	}
	fc += "</body></html>"

	return []byte(fc)
}

func processTgDelete(chatId int64, messageIds []int64) []byte {

	var fullContentJ []interface{}
	for _, messageId := range messageIds {
		upd, err := FindUpdateNewMessage(messageId)
		if err != nil {
			m := structs.MessageError{T: "Error", MessageId: messageId, Error: fmt.Sprintf("Error: %s", err)}
			fullContentJ = append(fullContentJ, m)

			continue
		}

		m := parseUpdateNewMessage(upd)
		m.T = "Deleted Message"
		fullContentJ = append(fullContentJ, parseUpdateNewMessage(upd))
	}
	j, _ := json.Marshal(fullContentJ)

	return j
}

func processTgEdit(chatId int64, messageId int64) []byte {
	var fullContentJ []interface{}

	updates, updateTypes, _, err := FindAllMessageChanges(messageId)
	if err != nil {
		m := structs.MessageError{T: "Error", MessageId: messageId, Error: fmt.Sprintf("Error: %s", err)}
		fullContentJ = append(fullContentJ, m)
		j, _ := json.Marshal(fullContentJ)

		return j
	}

	for i, rawJsonBytes := range updates {
		switch updateTypes[i] {
		case "updateNewMessage":
			upd, _ := client.UnmarshalUpdateNewMessage(rawJsonBytes)
			fullContentJ = append(fullContentJ, parseUpdateNewMessage(upd))
			break
		case "updateMessageEdited":
			upd, _ := client.UnmarshalUpdateMessageEdited(rawJsonBytes)
			fullContentJ = append(fullContentJ, parseUpdateMessageEdited(upd))
			break
		case "updateMessageContent":
			upd, _ := client.UnmarshalUpdateMessageContent(rawJsonBytes)
			fullContentJ = append(fullContentJ, parseUpdateMessageContent(upd))
			break
		default:
			m := structs.MessageError{T:"Error", MessageId: messageId, Error: fmt.Sprintf("Unknown update type: %s", updateTypes[i])}
			fullContentJ = append(fullContentJ, m)
		}
	}
	j, _ := json.Marshal(fullContentJ)

	return j
}

func processTgChat(chatId int64) []byte {
	var chat interface{}
	var err error
	if chatId > 0 {
		chat, err = GetUser(int32(chatId))
	} else{
		chat, err = GetChat(chatId)
	}
	if err != nil {

		return []byte("Error: " + err.Error())
	}
	j, _ := json.Marshal(chat)

	return j
}

func parseUpdateMessageEdited(upd *client.UpdateMessageEdited) structs.MessageEditedMeta {
	m := structs.MessageEditedMeta{
		T:         "Meta",
		MessageId: upd.MessageId,
		Date:      upd.EditDate,
		DateStr:   FormatTime(upd.EditDate),
	}

	return m
}

func parseUpdateNewMessage(upd *client.UpdateNewMessage) structs.MessageInfo {
	content := GetContent(upd.Message.Content)

	senderChatId := GetChatIdBySender(upd.Message.Sender)

	result := structs.MessageInfo{
		T:          "NewMessage",
		MessageId:  upd.Message.Id,
		Date:       upd.Message.Date,
		DateStr:    FormatTime(upd.Message.Date),
		ChatId:     upd.Message.ChatId,
		ChatName:   GetChatName(upd.Message.ChatId),
		SenderId:   senderChatId,
		SenderName: GetSenderName(upd.Message.Sender),
		Content:    content,
		ContentRaw: nil,
	}
	if verbose {
		result.ContentRaw = upd.Message.Content
	}

	return result
}

func parseUpdateMessageContent(upd *client.UpdateMessageContent) structs.MessageNewContent {
	result := structs.MessageNewContent{
		T:          "NewContent",
		MessageId:  upd.MessageId,
		Content:    GetContent(upd.NewContent),
		ContentRaw: nil,
	}
	if verbose {
		result.ContentRaw = upd.NewContent
	}

	return result
}

func formatNewMessageLink(upd *client.UpdateNewMessage) string {
	chat, _ := GetChat(upd.Message.ChatId)
	if upd.Message.Sender.MessageSenderType() == "messageSenderChat" {
		return fmt.Sprintf(`<a href="/e/%d/%d">message</a> in channel <a href="/c/%d">%s</a>`, upd.Message.ChatId, upd.Message.Id, chat.Id, chat.Title)
	} else {
		return fmt.Sprintf(`<a href="/e/%d/%d">message</a> from <a href="/c/%d">%s</a> in chat <a href="/c/%d">%s</a>`, upd.Message.ChatId, upd.Message.Id, GetChatIdBySender(upd.Message.Sender), GetSenderName(upd.Message.Sender), chat.Id, chat.Title)
	}
}

func formatDeletedMessagesLink(upd *client.UpdateDeleteMessages) string {
	chat, _ := GetChat(upd.ChatId)

	return fmt.Sprintf(`<a href="/d/%d/%s">%d messages</a> from chat <a href="/c/%d">%s</a>`, upd.ChatId, ImplodeInt(upd.MessageIds), len(upd.MessageIds), chat.Id, chat.Title)
}

func formatUpdatedContentLink(upd *client.UpdateMessageContent) string {
	chat, _ := GetChat(upd.ChatId)
	m, err := FindUpdateNewMessage(upd.MessageId)
	if err != nil {

		return fmt.Sprintf(`<a href="/e/%d/%d">message</a> in chat <a href="/c/%d">%s</a>`, upd.ChatId, upd.MessageId, chat.Id, chat.Title)
	}

	if m.Message.Sender.MessageSenderType() == "messageSenderChat" {
		return fmt.Sprintf(`<a href="/e/%d/%d">message</a> in channel <a href="/c/%d">%s</a>`, m.Message.ChatId, m.Message.Id, chat.Id, chat.Title)
	} else {
		return fmt.Sprintf(`<a href="/e/%d/%d">message</a> from <a href="/c/%d">%s</a> in chat <a href="/c/%d">%s</a>`, m.Message.ChatId, m.Message.Id, GetChatIdBySender(m.Message.Sender), GetSenderName(m.Message.Sender), chat.Id, chat.Title)
	}
}
