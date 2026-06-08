package utils

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

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

func ExtractYoutubeID(urlStr string) string {
	if idx := strings.Index(urlStr, "&si="); idx != -1 {
		urlStr = urlStr[:idx]
	}

	patterns := []string{
		`youtu\.be/([a-zA-Z0-9_-]+)`,
		`youtube\.com/watch\?v=([a-zA-Z0-9_-]+)`,
		`youtube\.com/embed/([a-zA-Z0-9_-]+)`,
		`youtube\.com/v/([a-zA-Z0-9_-]+)`,
		`youtube\.com/shorts/([a-zA-Z0-9_-]+)`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(urlStr)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	parsed, err := url.Parse(urlStr)
	if err == nil {
		if v := parsed.Query().Get("v"); v != "" {
			return v
		}
	}
	return ""
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
		}
	}

	return youtubeInfo, nil
}

func GetYoutubeStream(url string) (*YoutubeStream, error) {
	client := dlkitgo.NewClient()
	stream, err := client.Youtube.Stream(url)
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

	var bestVideoURL string
	var bestVideoQuality int
	var bestAudioURL string
	var bestAudioBitrate int

	for _, source := range stream.Source {
		if source.Type == "video" {
			quality := extractQuality(source.Quality)
			if quality > 0 && quality <= 720 && quality > bestVideoQuality {
				bestVideoQuality = quality
				bestVideoURL = source.URL
			}
		} else if source.Type == "audio" {
			bitrate := extractBitrate(source.Quality)
			if bitrate > bestAudioBitrate {
				bestAudioBitrate = bitrate
				bestAudioURL = source.URL
			}
		}
	}

	if bestVideoURL == "" {
		for _, source := range stream.Source {
			if source.Type == "video" {
				bestVideoURL = source.URL
				break
			}
		}
	}

	if bestAudioURL == "" {
		for _, source := range stream.Source {
			if source.Type == "audio" {
				bestAudioURL = source.URL
				break
			}
		}
	}

	youtubeStream.VideoURL = bestVideoURL
	youtubeStream.AudioURL = bestAudioURL

	return youtubeStream, nil
}

func extractQuality(quality string) int {
	re := regexp.MustCompile(`(\d+)p`)
	matches := re.FindStringSubmatch(quality)
	if len(matches) > 1 {
		q, _ := strconv.Atoi(matches[1])
		return q
	}
	return 0
}

func extractBitrate(quality string) int {
	re := regexp.MustCompile(`(\d+)kbps`)
	matches := re.FindStringSubmatch(quality)
	if len(matches) > 1 {
		b, _ := strconv.Atoi(matches[1])
		return b
	}
	return 0
}

func DownloadFileToTemp(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
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
	url := ExtractFirstURL(text)

	if url == "" {
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

	url = strings.Split(url, "&si=")[0]
	videoID := ExtractYoutubeID(url)
	if videoID == "" {
		ctx.EffectiveMessage.Reply(b, "❌ Could not extract YouTube ID. Please check the link.", nil)
		return nil
	}

	info, err := GetYoutubeInfo(url)
	if err != nil {
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	if info.Type == "playlist" {
		ctx.EffectiveMessage.Reply(b, "❌ Playlists are not supported. Please send a video or shorts link only.", nil)
		return nil
	}

	return handleYoutubeVideo(b, ctx, url, userID, chatID)
}

func handleYoutubeVideo(b *gotgbot.Bot, ctx *ext.Context, url string, userID, chatID int64) error {
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
			{Text: "🎥 Video (MP4)", CallbackData: fmt.Sprintf("yt_video_%d_%s", userID, ExtractYoutubeID(url))},
			{Text: "🎵 Audio (MP3)", CallbackData: fmt.Sprintf("yt_audio_%d_%s", userID, ExtractYoutubeID(url))},
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

	parts := strings.Split(query.Data, "_")
	if len(parts) < 4 {
		return nil
	}

	action := parts[1]
	userID, _ := strconv.ParseInt(parts[2], 10, 64)
	videoID := parts[3]

	if userID != query.From.Id {
		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "You can only download your own requests."})
		return nil
	}

	query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{})

	if action == "video" {
		go downloadYoutubeVideo(b, query, videoID)
	} else if action == "audio" {
		go downloadYoutubeAudio(b, query, videoID)
	}

	return nil
}

func downloadYoutubeVideo(b *gotgbot.Bot, query *gotgbot.CallbackQuery, videoID string) {
	inlineMsgID := query.InlineMessageId
	if inlineMsgID == "" && query.Message != nil {
		inlineMsgID = fmt.Sprintf("%d_%d", query.Message.GetChat().Id, query.Message.GetMessageId())
	}
	if inlineMsgID == "" {
		return
	}

	url := fmt.Sprintf("https://youtu.be/%s", videoID)
	stream, err := GetYoutubeStream(url)
	if err != nil || stream.VideoURL == "" {
		b.EditMessageText("❌ Failed to fetch video. Please try again.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}

	tempPath, err := DownloadFileToTemp(stream.VideoURL)
	if err != nil {
		b.EditMessageText("❌ Failed to download video. Please try again.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}
	defer os.Remove(tempPath)

	videoFile, err := os.Open(tempPath)
	if err != nil {
		b.EditMessageText("❌ Failed to open video file.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}
	defer videoFile.Close()

	durationMin := stream.Duration / 60
	durationSec := stream.Duration % 60
	caption := fmt.Sprintf("🎬 <b>%s</b>\n\n⏱️ <b>Duration:</b> %d:%02d", stream.Title, durationMin, durationSec)

	videoInput := gotgbot.InputMediaVideo{
		Media:     gotgbot.InputFileByReader(tempPath, videoFile),
		Caption:   caption,
		ParseMode: "HTML",
	}

	_, _, err = b.EditMessageMedia(videoInput, &gotgbot.EditMessageMediaOpts{
		InlineMessageId: inlineMsgID,
	})
	if err != nil {
		b.EditMessageText("❌ Failed to send video.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
	}
}

func downloadYoutubeAudio(b *gotgbot.Bot, query *gotgbot.CallbackQuery, videoID string) {
	inlineMsgID := query.InlineMessageId
	if inlineMsgID == "" && query.Message != nil {
		inlineMsgID = fmt.Sprintf("%d_%d", query.Message.GetChat().Id, query.Message.GetMessageId())
	}
	if inlineMsgID == "" {
		return
	}

	url := fmt.Sprintf("https://youtu.be/%s", videoID)
	stream, err := GetYoutubeStream(url)
	if err != nil || stream.AudioURL == "" {
		b.EditMessageText("❌ Failed to fetch audio. Please try again.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}

	tempPath, err := DownloadFileToTemp(stream.AudioURL)
	if err != nil {
		b.EditMessageText("❌ Failed to download audio. Please try again.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}
	defer os.Remove(tempPath)

	audioFile, err := os.Open(tempPath)
	if err != nil {
		b.EditMessageText("❌ Failed to open audio file.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}
	defer audioFile.Close()

	durationMin := stream.Duration / 60
	durationSec := stream.Duration % 60
	caption := fmt.Sprintf("🎵 <b>%s</b>\n\n⏱️ <b>Duration:</b> %d:%02d", stream.Title, durationMin, durationSec)

	audioInput := gotgbot.InputMediaAudio{
		Media:     gotgbot.InputFileByReader(tempPath, audioFile),
		Caption:   caption,
		ParseMode: "HTML",
		Title:     stream.Title,
		Performer: "YouTube",
		Duration:  int64(stream.Duration),
	}

	_, _, err = b.EditMessageMedia(audioInput, &gotgbot.EditMessageMediaOpts{
		InlineMessageId: inlineMsgID,
	})
	if err != nil {
		b.EditMessageText("❌ Failed to send audio.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
	}
}
