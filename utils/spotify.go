package utils

import (
	"fmt"
	"sync"
	"time"

	"github.com/Beesonn/dlkitgo"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var (
	activeTasks   = make(map[int64]bool)
	tasksMutex    = &sync.RWMutex{}
)

func isUserProcessing(userID int64) bool {
	tasksMutex.RLock()
	defer tasksMutex.RUnlock()
	return activeTasks[userID]
}

func setUserProcessing(userID int64, processing bool) {
	tasksMutex.Lock()
	defer tasksMutex.Unlock()
	if processing {
		activeTasks[userID] = true
	} else {
		delete(activeTasks, userID)
	}
}

func HandleSpotify(b *gotgbot.Bot, ctx *ext.Context) error {
	userID := ctx.EffectiveMessage.From.Id
	chatID := ctx.EffectiveChat.Id
	url := ctx.EffectiveMessage.Text

	if isUserProcessing(userID) {
		_, err := ctx.EffectiveMessage.Reply(b, 
			"You already have a task processing. Please wait for it to finish before sending another request.", 
			nil)
		return err
	}

	setUserProcessing(userID, true)
	defer setUserProcessing(userID, false)

	statusMsg, err := ctx.EffectiveMessage.Reply(b, "🎵 Processing Spotify link...", nil)
	if err != nil {
		return err
	}

	client := dlkitgo.NewClient()
	
	stream, err := client.Spotify.Stream(url)
	if err != nil {
		statusMsg.Delete(b, nil)
		_, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error processing Spotify link: %v", err), nil)
		return err
	}

	if len(stream.Source) == 0 {
		statusMsg.Delete(b, nil)
		_, err = ctx.EffectiveMessage.Reply(b, "❌ No tracks found in this Spotify link.", nil)
		return err
	}

	contentType := getSpotifyContentType(stream)
	trackCount := len(stream.Source)
	
	statusText := fmt.Sprintf("📥 Found %d track(s) (%s)\nStarting download...", trackCount, contentType)
	statusMsg.EditText(b, statusText, nil)

	switch contentType {
	case "playlist", "album":
		err = handleMultipleSpotifyTracks(b, ctx, stream, statusMsg)
	default:
		err = handleSingleSpotifyTrack(b, ctx, stream, statusMsg)
	}

	return err
}

func getSpotifyContentType(stream *dlkitgo.StreamResult) string {
	if len(stream.Source) == 0 {
		return "unknown"
	}
	
	if len(stream.Source) > 1 {
		return "playlist/album"
	}
	return "track"
}

func handleSingleSpotifyTrack(b *gotgbot.Bot, ctx *ext.Context, stream *dlkitgo.StreamResult, statusMsg *gotgbot.Message) error {
	source := stream.Source[0]
	
	statusMsg.EditText(b, fmt.Sprintf("📥 Downloading: %s - %s", source.Artist, source.Title), nil)

	audio := gotgbot.InputMediaAudio{
		Media:     gotgbot.InputFileByURL(source.URL),
		Caption:   fmt.Sprintf("%s - %s", source.Artist, source.Title),
		Title:     source.Title,
		Performer: source.Artist,
		Duration:  source.Duration,
	}

	if source.Image != "" {
		audio.Thumbnail = &gotgbot.InputFileByURL{FileURL: source.Image}
	}

	_, err := b.SendAudio(ctx.EffectiveChat.Id, audio.Media, &gotgbot.SendAudioOpts{
		Caption:   audio.Caption,
		Title:     audio.Title,
		Performer: audio.Performer,
		Duration:  audio.Duration,
		Thumbnail: audio.Thumbnail,
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId: ctx.EffectiveMessage.MessageId,
		},
	})

	statusMsg.Delete(b, nil)
	return err
}

func handleMultipleSpotifyTracks(b *gotgbot.Bot, ctx *ext.Context, stream *dlkitgo.StreamResult, statusMsg *gotgbot.Message) error {
	totalTracks := len(stream.Source)
	
	for i, source := range stream.Source {
		progressMsg := fmt.Sprintf("📥 Downloading %d/%d: %s - %s", 
			i+1, totalTracks, source.Artist, source.Title)
		statusMsg.EditText(b, progressMsg, nil)

		audio := gotgbot.InputMediaAudio{
			Media:     gotgbot.InputFileByURL(source.URL),
			Caption:   fmt.Sprintf("%s - %s", source.Artist, source.Title),
			Title:     source.Title,
			Performer: source.Artist,
			Duration:  source.Duration,
		}

		if source.Image != "" {
			audio.Thumbnail = &gotgbot.InputFileByURL{FileURL: source.Image}
		}

		err := sendWithFloodWait(b, ctx, audio, i+1, totalTracks)
		if err != nil {
			statusMsg.EditText(b, fmt.Sprintf("❌ Error at track %d: %v", i+1, err), nil)
			return err
		}

		if i < totalTracks-1 {
			time.Sleep(2 * time.Second)
		}
	}

	statusMsg.EditText(b, fmt.Sprintf("✅ Successfully uploaded %d track(s)!", totalTracks), nil)
	return nil
}

func sendWithFloodWait(b *gotgbot.Bot, ctx *ext.Context, audio gotgbot.InputMediaAudio, current, total int) error {
	maxRetries := 3
	retryDelay := 5 * time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		_, err := b.SendAudio(ctx.EffectiveChat.Id, audio.Media, &gotgbot.SendAudioOpts{
			Caption:   audio.Caption,
			Title:     audio.Title,
			Performer: audio.Performer,
			Duration:  audio.Duration,
			Thumbnail: audio.Thumbnail,
			ReplyParameters: &gotgbot.ReplyParameters{
				MessageId: ctx.EffectiveMessage.MessageId,
			},
		})

		if err == nil {
			return nil
		}

		if isFloodWaitError(err) {
			waitTime := extractFloodWaitTime(err)
			if waitTime > 0 {
				time.Sleep(time.Duration(waitTime) * time.Second)
			} else {
				time.Sleep(retryDelay)
			}
			continue
		}

		return err
	}

	return fmt.Errorf("failed after %d attempts due to flood wait", maxRetries)
}

func isFloodWaitError(err error) bool {
	errStr := err.Error()
	return containsAny(errStr, []string{"Flood", "Too Many Requests", "429"})
}

func extractFloodWaitTime(err error) int {
	return 5
}

func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if contains(s, substr) {
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
