package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/Beesonn/dlkitgo"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var (
	httpClient = &http.Client{}
)

type YoutubeVideo struct {
	Name     string
	Duration int
	URL      string
	VideoID  string
}

type YoutubeInfo struct {
	Type        string
	ID          string
	Name        string
	Image       string
	TotalVideos int
	Videos      []YoutubeVideo
}

type YoutubeStream struct {
	Title     string
	Duration  int
	Thumbnail string
	VideoURL  string
	AudioURL  string
}

func GetYoutubeInfo(rawURL string) (*YoutubeInfo, error) {
	client := dlkitgo.NewClient()
	info, err := client.Youtube.GetInfo(rawURL)
	if err != nil {
		return nil, fmt.Errorf("GetInfo failed: %v", err)
	}

	youtubeInfo := &YoutubeInfo{
		Type:        info.Type,
		ID:          info.ID,
		Name:        EscapeHTML(info.Name),
		Image:       info.Image,
		TotalVideos: 0,
		Videos:      []YoutubeVideo{},
	}

	if info.Type == "video" || info.Type == "shorts" {
		if len(info.Videos) > 0 {
			youtubeInfo.TotalVideos = 1
			youtubeInfo.Videos = append(youtubeInfo.Videos, YoutubeVideo{
				Name:     EscapeHTML(info.Name),
				Duration: info.Videos[0].Duration,
				URL:      rawURL,
				VideoID:  info.ID,
			})
		} else if info.ID != "" {
			youtubeInfo.TotalVideos = 1
			youtubeInfo.Videos = append(youtubeInfo.Videos, YoutubeVideo{
				Name:     EscapeHTML(info.Name),
				Duration: 0,
				URL:      rawURL,
				VideoID:  info.ID,
			})
		}
	}

	return youtubeInfo, nil
}

func GetYoutubeStream(rawURL string) (*YoutubeStream, error) {
	client := dlkitgo.NewClient()
	stream, err := client.Youtube.Stream(rawURL)
	if err != nil {
		return nil, err
	}

	youtubeStream := &YoutubeStream{
		Title:     EscapeHTML(stream.Caption),
		Duration:  stream.Duration,
		Thumbnail: stream.Thumbnail,
		VideoURL:  "",
		AudioURL:  "",
	}

	for _, source := range stream.Source {
		if source.Type == "video" {
			if youtubeStream.VideoURL == "" {
				youtubeStream.VideoURL = source.URL
			}
		} else if source.Type == "audio" {
			if youtubeStream.AudioURL == "" {
				youtubeStream.AudioURL = source.URL
			}
		}
	}

	return youtubeStream, nil
}

func DownloadFileToTemp(downloadURL string) (string, error) {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("bad status: %s", resp.Status)
	}

	tempFile, err := os.CreateTemp("", "youtube_*")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

func HandleYoutube(b *gotgbot.Bot, ctx *ext.Context) error {
	text := ctx.EffectiveMessage.Text
	rawURL := ExtractFirstURL(text)

	if rawURL == "" {
		ctx.EffectiveMessage.Reply(b, "❌ No valid YouTube link found in the message.", nil)
		return nil
	}

	userID := ctx.EffectiveMessage.From.Id
	chatID := ctx.EffectiveChat.Id

	if isUserProcessing(userID) {
		_, err := ctx.EffectiveMessage.Reply(b,
			"⚠️ You already have an ongoing task. Please wait for it to finish before sending another request.",
			nil)
		return err
	}

	info, err := GetYoutubeInfo(rawURL)
	if err != nil {
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	if info.Type == "playlist" {
		ctx.EffectiveMessage.Reply(b, "❌ Playlists are not supported. Please send a video or shorts link only.", nil)
		return nil
	}

	return handleYoutubeVideo(b, ctx, rawURL, userID, chatID, info.ID)
}

func handleYoutubeVideo(b *gotgbot.Bot, ctx *ext.Context, url string, userID, chatID int64, videoID string) error {
	statusMsg, err := ctx.EffectiveMessage.Reply(b, "🎬 Processing YouTube video...", nil)
	if err != nil {
		return err
	}

	stream, err := GetYoutubeStream(url)
	if err != nil {
		statusMsg.Delete(b, nil)
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	statusMsg.Delete(b, nil)

	durationMin := stream.Duration / 60
	durationSec := stream.Duration % 60

	text := fmt.Sprintf("🎬 <b>%s</b>\n\n⏱️ <b>Duration:</b> %d:%02d\n\n🔽 <b>Choose download format:</b>", stream.Title, durationMin, durationSec)

	keyboard := [][]gotgbot.InlineKeyboardButton{
		{
			{Text: "🎥 Video (MP4)", CallbackData: fmt.Sprintf("yt#%d#%s#video", userID, videoID)},
			{Text: "🎵 Audio (MP3)", CallbackData: fmt.Sprintf("yt#%d#%s#audio", userID, videoID)},
		},
		{
			{Text: "❌ Cancel", CallbackData: "cancel"},
		},
	}

	replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}

	_, err = ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: replyMarkup,
	})
	return err
}

func HandleYoutubeCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.Update.CallbackQuery
	if query == nil {
		return nil
	}

	data := query.Data
	if !strings.HasPrefix(data, "yt#") {
		return nil
	}

	if query.InlineMessageId != "" {
		return handleYoutubeInlineCallback(b, query, data)
	}

	return handleYoutubeNormalCallback(b, query, data)
}

func handleYoutubeNormalCallback(b *gotgbot.Bot, query *gotgbot.CallbackQuery, data string) error {
	parts := strings.Split(data, "#")
	if len(parts) != 4 {
		return nil
	}

	userID, _ := strconv.ParseInt(parts[1], 10, 64)
	videoID := parts[2]
	action := parts[3]

	if userID != query.From.Id {
		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "You can only download your own requests."})
		return nil
	}

	query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{})

	chatID := query.Message.GetChat().Id
	messageID := query.Message.GetMessageId()
	videoURL := fmt.Sprintf("https://youtu.be/%s", videoID)

	if action == "video" {
		go downloadYoutubeNormal(b, videoURL, action, chatID, messageID)
	} else if action == "audio" {
		go downloadYoutubeNormal(b, videoURL, action, chatID, messageID)
	}

	return nil
}

func handleYoutubeInlineCallback(b *gotgbot.Bot, query *gotgbot.CallbackQuery, data string) error {
	parts := strings.Split(data, "#")
	if len(parts) != 4 {
		return nil
	}

	userID, _ := strconv.ParseInt(parts[1], 10, 64)
	videoID := parts[2]
	action := parts[3]

	if userID != query.From.Id {
		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "You can only download your own requests."})
		return nil
	}

	query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text:      "Downloading... Please wait",
		ShowAlert: true,
	})

	inlineMsgID := query.InlineMessageId
	videoURL := fmt.Sprintf("https://youtu.be/%s", videoID)

	if action == "video" {
		go downloadYoutubeInline(b, videoURL, action, inlineMsgID)
	} else if action == "audio" {
		go downloadYoutubeInline(b, videoURL, action, inlineMsgID)
	}

	return nil
}

func downloadYoutubeNormal(b *gotgbot.Bot, videoURL, format string, chatID, messageID int64) {
	var statusMsg *gotgbot.Message
	var err error

	if format == "video" {
		statusMsg, err = b.SendMessage(chatID, "🎬 <b>Downloading video...</b>\n\nPlease wait...", &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
	} else {
		statusMsg, err = b.SendMessage(chatID, "🎵 <b>Downloading audio...</b>\n\nPlease wait...", &gotgbot.SendMessageOpts{
			ParseMode: "HTML",
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
	}
	if err != nil {
		return
	}

	stream, err := GetYoutubeStream(videoURL)
	if err != nil {
		statusMsg.Delete(b, nil)
		b.SendMessage(chatID, "❌ Failed to fetch media. Please try again.", &gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
		return
	}

	var downloadURL string
	if format == "video" {
		downloadURL = stream.VideoURL
	} else {
		downloadURL = stream.AudioURL
	}

	if downloadURL == "" {
		statusMsg.Delete(b, nil)
		b.SendMessage(chatID, fmt.Sprintf("❌ No %s stream available. Please try again.", format), &gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
		return
	}

	tempPath, err := DownloadFileToTemp(downloadURL)
	if err != nil {
		statusMsg.Delete(b, nil)
		b.SendMessage(chatID, fmt.Sprintf("❌ Failed to download %s. Please try again.", format), &gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
		return
	}
	defer os.Remove(tempPath)

	file, err := os.Open(tempPath)
	if err != nil {
		statusMsg.Delete(b, nil)
		b.SendMessage(chatID, fmt.Sprintf("❌ Failed to open %s file.", format), &gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
		return
	}
	defer file.Close()

	durationMin := stream.Duration / 60
	durationSec := stream.Duration % 60
	caption := fmt.Sprintf("🎬 <b>%s</b>\n\n⏱️ <b>Duration:</b> %d:%02d", stream.Title, durationMin, durationSec)

	statusMsg.Delete(b, nil)

	if format == "video" {
		_, err = b.SendVideo(chatID, gotgbot.InputFileByReader(tempPath, file), &gotgbot.SendVideoOpts{
			Caption:   caption,
			ParseMode: "HTML",
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
	} else {
		_, err = b.SendAudio(chatID, gotgbot.InputFileByReader(tempPath, file), &gotgbot.SendAudioOpts{
			Caption:   caption,
			ParseMode: "HTML",
			Title:     stream.Title,
			Performer: "YouTube",
			Duration:  int64(stream.Duration),
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
	}

	if err != nil {
		b.SendMessage(chatID, fmt.Sprintf("❌ Failed to send %s.", format), &gotgbot.SendMessageOpts{
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: messageID,
			},
		})
	}
}

func downloadYoutubeInline(b *gotgbot.Bot, videoURL, format string, inlineMsgID string) {
	if format == "video" {
		b.EditMessageText("🎬 <b>Downloading video...</b>\n\nPlease wait...", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
			ParseMode:       "HTML",
		})
	} else {
		b.EditMessageText("🎵 <b>Downloading audio...</b>\n\nPlease wait...", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
			ParseMode:       "HTML",
		})
	}

	stream, err := GetYoutubeStream(videoURL)
	if err != nil {
		b.EditMessageText("❌ Failed to fetch media. Please try again.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}

	var downloadURL string
	if format == "video" {
		downloadURL = stream.VideoURL
	} else {
		downloadURL = stream.AudioURL
	}

	if downloadURL == "" {
		b.EditMessageText(fmt.Sprintf("❌ No %s stream available. Please try again.", format), &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}

	tempPath, err := DownloadFileToTemp(downloadURL)
	if err != nil {
		b.EditMessageText(fmt.Sprintf("❌ Failed to download %s. Please try again.", format), &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}
	defer os.Remove(tempPath)

	file, err := os.Open(tempPath)
	if err != nil {
		b.EditMessageText(fmt.Sprintf("❌ Failed to open %s file.", format), &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}
	defer file.Close()

	durationMin := stream.Duration / 60
	durationSec := stream.Duration % 60
	caption := fmt.Sprintf("🎬 <b>%s</b>\n\n⏱️ <b>Duration:</b> %d:%02d", stream.Title, durationMin, durationSec)

	if format == "video" {
		videoInput := gotgbot.InputMediaVideo{
			Media:     gotgbot.InputFileByReader(tempPath, file),
			Caption:   caption,
			ParseMode: "HTML",
		}
		_, _, err = b.EditMessageMedia(videoInput, &gotgbot.EditMessageMediaOpts{
			InlineMessageId: inlineMsgID,
		})
	} else {
		audioInput := gotgbot.InputMediaAudio{
			Media:     gotgbot.InputFileByReader(tempPath, file),
			Caption:   caption,
			ParseMode: "HTML",
			Title:     stream.Title,
			Performer: "YouTube",
			Duration:  int64(stream.Duration),
		}
		_, _, err = b.EditMessageMedia(audioInput, &gotgbot.EditMessageMediaOpts{
			InlineMessageId: inlineMsgID,
		})
	}

	if err != nil {
		b.EditMessageText(fmt.Sprintf("❌ Failed to send %s.", format), &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
	}
}
