package bot

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Beesonn/MediaSaveBot/utils"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func GetSearchResults(b *gotgbot.Bot, ctx *ext.Context, query string) error {
	userID := ctx.EffectiveUser.Id

	statusMsg, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("🔍 Searching for: <b>%s</b>\n\nPlease wait...", utils.EscapeHTML(query)), &gotgbot.SendMessageOpts{ParseMode: "HTML"})
	if err != nil {
		return err
	}

	searchResults, err := utils.SearchSpotifyTracks(query)
	if err != nil {
		statusMsg.Delete(b, nil)
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return err
	}

	if len(searchResults) == 0 {
		statusMsg.Delete(b, nil)
		ctx.EffectiveMessage.Reply(b, "❌ No results found. Please try different keywords.", nil)
		return nil
	}

	statusMsg.Delete(b, nil)
	messageText := fmt.Sprintf("🎵 <b>Search Results for: %s</b>\n\n", utils.EscapeHTML(query))

	keyboard := make([][]gotgbot.InlineKeyboardButton, 0)
	for i, track := range searchResults {
		if i >= 20 {
			break
		}
		songName := track.Name
		if len(songName) > 35 {
			songName = songName[:32] + "..."
		}
		artistName := track.Artists
		if len(artistName) > 20 {
			artistName = artistName[:17] + "..."
		}
		buttonText := fmt.Sprintf("%d. %s - %s", i+1, songName, artistName)
		keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
			Text:         buttonText,
			CallbackData: fmt.Sprintf("song_%d_%s", userID, track.ID),
		}})
	}

	keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
		Text:         "❌ Cancel",
		CallbackData: "song_cancel",
	}})

	replyMarkup := &gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}
	_, err = ctx.EffectiveMessage.Reply(b, messageText, &gotgbot.SendMessageOpts{ParseMode: "HTML", ReplyMarkup: replyMarkup})
	return err
}

func HandleSong(b *gotgbot.Bot, ctx *ext.Context) error {
	query := strings.TrimSpace(strings.TrimPrefix(ctx.EffectiveMessage.Text, "/song"))
	if query == "" {
		if ctx.EffectiveMessage.ReplyToMessage != nil && ctx.EffectiveMessage.ReplyToMessage.Text != "" {
			query = strings.TrimSpace(ctx.EffectiveMessage.ReplyToMessage.Text)
		}
	}
	if query == "" {
		_, err := ctx.EffectiveMessage.Reply(b,
			"🎵 <b>Please provide a song name!</b>\n\n<b>Usage:</b> <code>/song song name</code>\n<b>Example:</b> <code>/song never gonna give you up</code>\n\n<b>Or reply to a message with</b> <code>/song</code>",
			&gotgbot.SendMessageOpts{ParseMode: "HTML"})
		return err
	}
	return GetSearchResults(b, ctx, query)
}

func HandleSongCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.Update.CallbackQuery
	data := query.Data
	callerID := query.From.Id
	chatID := query.Message.GetChat().Id
	messageID := query.Message.GetMessageId()

	if data == "song_cancel" {
		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Cancelled"})
		b.DeleteMessage(chatID, messageID, nil)
		return nil
	}

	if !strings.HasPrefix(data, "song_") {
		return nil
	}

	parts := strings.Split(data, "_")
	if len(parts) != 3 {
		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Something went wrong"})
		return nil
	}

	userID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Something went wrong"})
		return nil
	}
	trackID := parts[2]

	if userID != callerID {
		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "⚠️ You can only download songs you searched for."})
		return nil
	}

	query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Downloading song...", ShowAlert: false})

	progressMsg, err := b.SendMessage(chatID, "🎵 <b>Downloading...</b>\n\nPlease wait...", &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId: messageID,
		},
	})
	if err != nil {
		return err
	}

	go downloadAndSendSong(b, chatID, progressMsg.MessageId, trackID)
	return nil
}

func downloadAndSendSong(b *gotgbot.Bot, chatID int64, progressMsgID int64, trackID string) {
	trackURL := fmt.Sprintf("https://open.spotify.com/track/%s", trackID)
	stream, err := utils.GetTrackStream(trackURL)
	if err != nil {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsgID})
		return
	}
	if len(stream.Source) == 0 {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsgID})
		return
	}
	source := stream.Source[0]
	caption := utils.FormatSongCaption(source.Title, source.Artist, source.Duration)
	audioOpts := &gotgbot.SendAudioOpts{
		Caption:   caption,
		Title:     source.Title,
		Performer: source.Artist,
		Duration:  int64(source.Duration),
		RequestOpts: &gotgbot.RequestOpts{
			Timeout: time.Minute * 50,
		},
		ParseMode: "HTML",
	}
	_, err = b.SendAudio(chatID, gotgbot.InputFileByURL(source.URL), audioOpts)
	if err != nil {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsgID})
		return
	}
	b.DeleteMessage(chatID, progressMsgID, nil)
}
