package bot

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/Beesonn/MediaSaveBot/utils"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func generateUUID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func HandleInlineQuery(b *gotgbot.Bot, ctx *ext.Context) error {
	inlineQuery := ctx.Update.InlineQuery
	if inlineQuery == nil {
		return nil
	}

	query := inlineQuery.Query
	userID := inlineQuery.From.Id
	botUsername := b.User.Username

	if query == "" {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "How to use this bot",
				Description: fmt.Sprintf("Use @%s to search songs, playlists, Instagram, Pinterest, YouTube", botUsername),
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: fmt.Sprintf("🎵 <b>Inline Mode Usage - @%s</b>\n\n"+
						"• <code>@%s song name</code> - Search and send audio\n"+
						"• <code>@%s Spotify playlist/album link</code> - Browse playlist\n"+
						"• <code>@%s Instagram link</code> - Send photo/video\n"+
						"• <code>@%s Pinterest link</code> - Send image/video\n"+
						"• <code>@%s YouTube link</code> - Download video/audio\n\n"+
						"<b>Examples:</b>\n"+
						"<code>@%s never gonna give you up</code>\n"+
						"<code>@%s https://open.spotify.com/playlist/xxx</code>\n"+
						"<code>@%s https://instagram.com/p/xxx</code>\n"+
						"<code>@%s https://youtu.be/xxx</code>",
						botUsername, botUsername, botUsername, botUsername, botUsername, botUsername,
						botUsername, botUsername, botUsername, botUsername),
					ParseMode: "HTML",
				},
				ReplyMarkup: &gotgbot.InlineKeyboardMarkup{
					InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
						{
							{Text: "🎵 Search Song", SwitchInlineQueryCurrentChat: &[]string{""}[0]},
						},
						{
							{Text: "👥 Support Group", Url: "https://t.me/XBOTSUPPORTS"},
							{Text: "📢 Update Channel", Url: "https://t.me/BeesonsBots"},
						},
					},
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{
			CacheTime: &cacheTime,
		})
		return err
	}

	if strings.HasPrefix(query, "https://") && (strings.Contains(query, "youtu.be") || strings.Contains(query, "youtube.com")) {
		return handleYoutubeInline(b, inlineQuery, query)
	}

	if strings.HasPrefix(query, "https://") && (strings.Contains(query, "instagram.com") || strings.Contains(query, "instagr.am")) {
		return handleInstagramInline(b, inlineQuery, query)
	}

	if strings.HasPrefix(query, "https://") && (strings.Contains(query, "pinterest.com") || strings.Contains(query, "pin.it")) {
		return handlePinterestInline(b, inlineQuery, query)
	}

	if strings.HasPrefix(query, "https://") && (strings.Contains(query, "playlist") || strings.Contains(query, "album")) {
		return handlePlaylistInline(b, inlineQuery, query, botUsername)
	}

	if strings.HasPrefix(query, "https://") && strings.Contains(query, "open.spotify.com") {
		return handleSpotifyTrackInline(b, inlineQuery, query)
	}

	return handleSongInlineFast(b, inlineQuery, query, userID)
}

func handleYoutubeInline(b *gotgbot.Bot, inlineQuery *gotgbot.InlineQuery, url string) error {
	cleanURL := strings.Split(url, "&si=")[0]
	videoID := utils.ExtractYoutubeID(cleanURL)
	if videoID == "" {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Invalid YouTube URL",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Invalid YouTube URL. Please send a valid YouTube link.",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	info, err := utils.GetYoutubeInfo(cleanURL)
	if err != nil {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Failed to fetch YouTube info",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	if info.Type == "playlist" {
		if info.TotalVideos == 0 {
			results := []gotgbot.InlineQueryResult{
				&gotgbot.InlineQueryResultArticle{
					Id:          generateUUID(),
					Title:       "No videos",
					Description: "Playlist is empty",
					InputMessageContent: &gotgbot.InputTextMessageContent{
						MessageText: "❌ No videos found in this playlist.",
						ParseMode:   "HTML",
					},
				},
			}
			cacheTime := int64(0)
			_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
			return err
		}
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       fmt.Sprintf("📺 %s", info.Name),
				Description: fmt.Sprintf("%d videos - Send to bot in private to browse", info.TotalVideos),
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: fmt.Sprintf("📺 <b>%s</b>\n\n📊 <b>Total videos:</b> %d\n\nSend this playlist link to me in a private chat to browse and download:\n\n<code>%s</code>", info.Name, info.TotalVideos, cleanURL),
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(300)
		_, err = inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	durationMin := info.Videos[0].Duration / 60
	durationSec := info.Videos[0].Duration % 60

	keyboard := [][]gotgbot.InlineKeyboardButton{
		{
			{Text: "🎥 Video (MP4)", CallbackData: fmt.Sprintf("yt_inline_video_%s", videoID)},
			{Text: "🎵 Audio (MP3)", CallbackData: fmt.Sprintf("yt_inline_audio_%s", videoID)},
		},
	}
	replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}

	result := &gotgbot.InlineQueryResultArticle{
		Id:          generateUUID(),
		Title:       info.Name,
		Description: fmt.Sprintf("Duration: %d:%02d", durationMin, durationSec),
		InputMessageContent: &gotgbot.InputTextMessageContent{
			MessageText: fmt.Sprintf("🎬 <b>%s</b>\n\n⏱️ <b>Duration:</b> %d:%02d\n\nChoose format:", info.Name, durationMin, durationSec),
			ParseMode:   "HTML",
		},
		ReplyMarkup: &replyMarkup,
	}

	cacheTime := int64(300)
	_, err = inlineQuery.Answer(b, []gotgbot.InlineQueryResult{result}, &gotgbot.AnswerInlineQueryOpts{
		CacheTime: &cacheTime,
	})
	return err
}

func HandleInlineYoutubeCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.Update.CallbackQuery
	if query == nil {
		return nil
	}

	parts := strings.Split(query.Data, "_")
	if len(parts) < 4 {
		return nil
	}

	action := parts[2]
	videoID := parts[3]

	_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text:      "Downloading...",
		ShowAlert: true,
	})
	if err != nil {
		return err
	}

	if action == "video" {
		go downloadAndSendVideo(b, query, videoID)
	} else if action == "audio" {
		go downloadAndSendAudio(b, query, videoID)
	}

	return nil
}

func downloadAndSendVideo(b *gotgbot.Bot, query *gotgbot.CallbackQuery, videoID string) {
	inlineMsgID := query.InlineMessageId
	if inlineMsgID == "" {
		return
	}

	url := fmt.Sprintf("https://youtu.be/%s", videoID)
	stream, err := utils.GetYoutubeStream(url)
	if err != nil || stream.VideoURL == "" {
		b.EditMessageText("❌ Failed to fetch video. Please try again.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}

	tempPath, err := utils.DownloadFileToTemp(stream.VideoURL)
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

func downloadAndSendAudio(b *gotgbot.Bot, query *gotgbot.CallbackQuery, videoID string) {
	inlineMsgID := query.InlineMessageId
	if inlineMsgID == "" {
		return
	}

	url := fmt.Sprintf("https://youtu.be/%s", videoID)
	stream, err := utils.GetYoutubeStream(url)
	if err != nil || stream.AudioURL == "" {
		b.EditMessageText("❌ Failed to fetch audio. Please try again.", &gotgbot.EditMessageTextOpts{
			InlineMessageId: inlineMsgID,
		})
		return
	}

	tempPath, err := utils.DownloadFileToTemp(stream.AudioURL)
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

func handleInstagramInline(b *gotgbot.Bot, inlineQuery *gotgbot.InlineQuery, url string) error {
	sources, caption, err := utils.GetInstagramMedia(url)
	if err != nil || len(sources) == 0 {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Failed to fetch Instagram media",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	results := make([]gotgbot.InlineQueryResult, 0)
	for _, s := range sources {
		source := s.(struct {
			Type string
			URL  string
		})
		id := generateUUID()
		if source.Type == "video" {
			result := &gotgbot.InlineQueryResultVideo{
				Id:       id,
				VideoUrl: source.URL,
				MimeType: "video/mp4",
				Title:    "Instagram Video",
				Caption:  caption,
			}
			results = append(results, result)
		} else {
			result := &gotgbot.InlineQueryResultPhoto{
				Id:       id,
				PhotoUrl: source.URL,
				Caption:  caption,
			}
			results = append(results, result)
		}
	}

	cacheTime := int64(300)
	_, err = inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{
		CacheTime: &cacheTime,
	})
	return err
}

func handlePinterestInline(b *gotgbot.Bot, inlineQuery *gotgbot.InlineQuery, url string) error {
	sources, title, err := utils.GetPinterestMedia(url)
	if err != nil || len(sources) == 0 {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Failed to fetch Pinterest media",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	results := make([]gotgbot.InlineQueryResult, 0)
	for _, s := range sources {
		source := s.(struct {
			Type string
			URL  string
		})
		id := generateUUID()
		if source.Type == "video" {
			result := &gotgbot.InlineQueryResultVideo{
				Id:       id,
				VideoUrl: source.URL,
				MimeType: "video/mp4",
				Title:    "Pinterest Video",
				Caption:  title,
			}
			results = append(results, result)
		} else {
			result := &gotgbot.InlineQueryResultPhoto{
				Id:       id,
				PhotoUrl: source.URL,
				Caption:  title,
			}
			results = append(results, result)
		}
	}

	cacheTime := int64(300)
	_, err = inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{
		CacheTime: &cacheTime,
	})
	return err
}

func handlePlaylistInline(b *gotgbot.Bot, inlineQuery *gotgbot.InlineQuery, url, botUsername string) error {
	playlistID := utils.ExtractSpotifyID(url)
	if playlistID == "" {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Invalid playlist URL",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Invalid Spotify URL. Please send a valid playlist or album link.",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	typ := "playlist"
	if strings.Contains(url, "/album/") {
		typ = "album"
	}

	info, err := utils.GetSpotifyInfoByID(playlistID, typ)
	if err != nil {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Failed to fetch playlist info",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	encodedID := utils.EncodePlaylistCallback(playlistID, typ)
	deepLink := fmt.Sprintf("https://t.me/%s?start=dl_%s", botUsername, encodedID)

	var title, emoji string
	if typ == "playlist" {
		emoji = "📀"
		title = "Playlist"
	} else {
		emoji = "💿"
		title = "Album"
	}

	messageText := fmt.Sprintf("%s <b>%s: %s</b>\n📊 <b>Total tracks:</b> %d\n\n🔽 <b>Click the button below to browse this %s</b>",
		emoji, title, utils.EscapeHTML(info.Name), info.TotalTracks, title)

	results := []gotgbot.InlineQueryResult{
		&gotgbot.InlineQueryResultArticle{
			Id:          generateUUID(),
			Title:       fmt.Sprintf("%s: %s", title, info.Name),
			Description: fmt.Sprintf("%d tracks - Click to browse", info.TotalTracks),
			InputMessageContent: &gotgbot.InputTextMessageContent{
				MessageText: messageText,
				ParseMode:   "HTML",
			},
			ReplyMarkup: &gotgbot.InlineKeyboardMarkup{
				InlineKeyboard: [][]gotgbot.InlineKeyboardButton{
					{
						{Text: "🎵 Browse Playlist", Url: deepLink},
					},
				},
			},
		},
	}

	cacheTime := int64(300)
	_, err = inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{
		CacheTime: &cacheTime,
	})
	return err
}

func handleSpotifyTrackInline(b *gotgbot.Bot, inlineQuery *gotgbot.InlineQuery, url string) error {
	trackID := utils.ExtractSpotifyID(url)
	if trackID == "" {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Invalid Spotify URL",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Invalid Spotify URL. Please send a valid Spotify link.",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	stream, err := utils.GetTrackStream(url)
	if err != nil || len(stream.Source) == 0 {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Failed to fetch track",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	source := stream.Source[0]
	result := &gotgbot.InlineQueryResultAudio{
		Id:            generateUUID(),
		AudioUrl:      source.URL,
		Title:         source.Title,
		Performer:     source.Artist,
		AudioDuration: int64(source.Duration),
		Caption:       fmt.Sprintf("<b>%s</b>\n\n🎤 <b>Artist:</b> %s\n⏱️ <b>Duration:</b> %d seconds", utils.EscapeHTML(source.Title), utils.EscapeHTML(source.Artist), source.Duration),
		ParseMode:     "HTML",
	}

	cacheTime := int64(300)
	_, err = inlineQuery.Answer(b, []gotgbot.InlineQueryResult{result}, &gotgbot.AnswerInlineQueryOpts{
		CacheTime: &cacheTime,
	})
	return err
}

func handleSongInlineFast(b *gotgbot.Bot, inlineQuery *gotgbot.InlineQuery, query string, userID int64) error {
	searchResults, err := utils.SearchSpotifyTracks(query)
	if err != nil {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "Error",
				Description: "Something went wrong. Please try again.",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	if len(searchResults) == 0 {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "No results found",
				Description: "Try different keywords",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ No results found for: " + utils.EscapeHTML(query) + "\n\nTry different keywords.",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	results := make([]gotgbot.InlineQueryResult, 0)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, track := range searchResults {
		wg.Add(1)
		go func(trackID, trackName, trackArtist string, trackDuration int) {
			defer wg.Done()

			trackURL := fmt.Sprintf("https://open.spotify.com/track/%s", trackID)
			stream, err := utils.GetTrackStream(trackURL)
			if err != nil || len(stream.Source) == 0 {
				return
			}
			source := stream.Source[0]

			result := &gotgbot.InlineQueryResultAudio{
				Id:            generateUUID(),
				AudioUrl:      source.URL,
				Title:         source.Title,
				Performer:     source.Artist,
				AudioDuration: int64(source.Duration),
				Caption:       fmt.Sprintf("<b>%s</b>\n\n🎤 <b>Artist:</b> %s\n⏱️ <b>Duration:</b> %d seconds", utils.EscapeHTML(source.Title), utils.EscapeHTML(source.Artist), source.Duration),
				ParseMode:     "HTML",
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(track.ID, track.Name, track.Artists, track.Duration)
	}

	wg.Wait()

	if len(results) == 0 {
		results := []gotgbot.InlineQueryResult{
			&gotgbot.InlineQueryResultArticle{
				Id:          generateUUID(),
				Title:       "No audio available",
				Description: "Could not fetch audio stream",
				InputMessageContent: &gotgbot.InputTextMessageContent{
					MessageText: "❌ Could not fetch audio stream for these tracks. Please try again.",
					ParseMode:   "HTML",
				},
			},
		}
		cacheTime := int64(0)
		_, err := inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{CacheTime: &cacheTime})
		return err
	}

	cacheTime := int64(300)
	isPersonal := true
	_, err = inlineQuery.Answer(b, results, &gotgbot.AnswerInlineQueryOpts{
		CacheTime:  &cacheTime,
		IsPersonal: isPersonal,
	})
	return err
}
