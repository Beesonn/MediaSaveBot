package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Beesonn/dlkitgo"
	"github.com/Beesonn/dlkitgo/spotify"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var (
	activeTasks     = make(map[int64]bool)
	tasksMutex      = &sync.RWMutex{}
	cancelDownloads = make(map[int64]chan bool)
	cancelMutex     = &sync.RWMutex{}
	playlistCache   = make(map[string]*CachedPlaylist)
	playlistCacheMu = &sync.RWMutex{}
	BotUsername     string
	failedTracks    = make(map[string]*FailedTrack)
	failedMutex     = &sync.RWMutex{}
)

type PlaylistTrack struct {
	Name     string
	Artist   string
	URL      string
	Duration int
	SongID   string
}

type PlaylistInfo struct {
	Type        string
	Name        string
	Artist      string
	Image       string
	TotalTracks int
	Tracks      []PlaylistTrack
	PlaylistID  string
}

type CachedPlaylist struct {
	Info      *PlaylistInfo
	ExpiresAt time.Time
}

type FailedTrack struct {
	Track      PlaylistTrack
	UserID     int64
	ChatID     int64
	RetryCount int
}

func SetBotUsername(username string) {
	BotUsername = username
}

func EncodePlaylistCallback(playlistID, typ string) string {
	return fmt.Sprintf("%s-%s", playlistID, typ)
}

func decodePlaylistCallback(encoded string) (string, string, error) {
	parts := strings.SplitN(encoded, "-", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid format")
	}
	return parts[0], parts[1], nil
}

func GetCancelChannel(userID int64) (bool, chan bool) {
	cancelMutex.RLock()
	defer cancelMutex.RUnlock()
	ch, exists := cancelDownloads[userID]
	return exists, ch
}

func RemoveCancelChannel(userID int64) {
	cancelMutex.Lock()
	defer cancelMutex.Unlock()
	delete(cancelDownloads, userID)
}

func SetCancelChannel(userID int64, ch chan bool) {
	cancelMutex.Lock()
	defer cancelMutex.Unlock()
	if oldCh, exists := cancelDownloads[userID]; exists {
		close(oldCh)
	}
	cancelDownloads[userID] = ch
}

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

func SearchSpotifyTracks(query string) ([]spotify.SearchResult, error) {
	client := dlkitgo.NewClient()
	search, err := client.Spotify.Search(query, "track")
	if err != nil {
		return nil, err
	}
	return search.Results, nil
}

func GetTrackStream(trackURL string) (spotify.StreamResult, error) {
	client := dlkitgo.NewClient()
	return client.Spotify.Stream(trackURL)
}

func ExtractSpotifyID(url string) string {
	url = strings.Split(url, "?")[0]
	patterns := []string{
		`playlist/([a-zA-Z0-9]+)`,
		`album/([a-zA-Z0-9]+)`,
		`track/([a-zA-Z0-9]+)`,
	}
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func EscapeHTML(s string) string {
	return html.EscapeString(s)
}

func GetSpotifyInfoByID(playlistID, typ string) (*PlaylistInfo, error) {
	if playlistID == "" {
		return nil, fmt.Errorf("empty playlist ID")
	}
	playlistID = strings.TrimSpace(playlistID)
	var url string
	switch typ {
	case "playlist":
		url = "https://open.spotify.com/playlist/" + playlistID
	case "album":
		url = "https://open.spotify.com/album/" + playlistID
	default:
		return nil, fmt.Errorf("unknown type: %s", typ)
	}
	client := dlkitgo.NewClient()
	info, err := client.Spotify.GetInfo(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch: %v", err)
	}
	playlistInfo := &PlaylistInfo{
		Type:        info.Type,
		Name:        EscapeHTML(info.Name),
		Artist:      EscapeHTML(info.Artist),
		Image:       info.Image,
		TotalTracks: len(info.Tracks),
		Tracks:      make([]PlaylistTrack, len(info.Tracks)),
		PlaylistID:  playlistID,
	}
	for i, track := range info.Tracks {
		playlistInfo.Tracks[i] = PlaylistTrack{
			Name:     EscapeHTML(track.Name),
			Artist:   EscapeHTML(track.Artist),
			URL:      track.URL,
			Duration: track.Duration,
			SongID:   ExtractSpotifyID(track.URL),
		}
	}
	return playlistInfo, nil
}

func FormatSongCaption(songName, artist string, duration int) string {
	return fmt.Sprintf("<b>%s</b>\n\n🎤 <b>Artist:</b> %s\n⏱️ <b>Duration:</b> %d seconds", EscapeHTML(songName), EscapeHTML(artist), duration)
}

func GenerateRandomID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func HandleSpotify(b *gotgbot.Bot, ctx *ext.Context) error {
	text := ctx.EffectiveMessage.Text
	url := ExtractFirstURL(text)

	if url == "" {
		ctx.EffectiveMessage.Reply(b, "❌ No valid Spotify link found in the message.", nil)
		return nil
	}

	userID := ctx.EffectiveMessage.From.Id
	chatID := ctx.EffectiveChat.Id
	chatType := ctx.EffectiveChat.Type

	if isUserProcessing(userID) {
		_, err := ctx.EffectiveMessage.Reply(b,
			"⚠️ You already have an ongoing task. Please wait for it to finish before sending another request.",
			nil)
		return err
	}

	url = strings.Split(url, "?")[0]
	playlistID := ExtractSpotifyID(url)
	if playlistID == "" {
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	typ := detectType(url)
	if typ == "track" {
		return handleSingleSpotifyTrackByURL(b, ctx, url, userID)
	}

	if typ == "playlist" || typ == "album" {
		info, err := GetSpotifyInfoByID(playlistID, typ)
		if err != nil {
			ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
			return nil
		}

		setUserProcessing(userID, true)
		defer setUserProcessing(userID, false)

		cacheKey := EncodePlaylistCallback(playlistID, typ)
		playlistCacheMu.Lock()
		playlistCache[cacheKey] = &CachedPlaylist{
			Info:      info,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
		playlistCacheMu.Unlock()

		return sendPlaylistPageMessage(b, chatID, info, 0, userID, cacheKey, ctx.EffectiveMessage.MessageId, chatType)
	}

	ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
	return nil
}

func detectType(url string) string {
	if strings.Contains(url, "/playlist/") {
		return "playlist"
	}
	if strings.Contains(url, "/album/") {
		return "album"
	}
	if strings.Contains(url, "/track/") {
		return "track"
	}
	return "unknown"
}

func handleSingleSpotifyTrackByURL(b *gotgbot.Bot, ctx *ext.Context, url string, userID int64) error {
	statusMsg, err := ctx.EffectiveMessage.Reply(b, "🎵 Processing Spotify track...", nil)
	if err != nil {
		return err
	}

	trackID := ExtractSpotifyID(url)
	if trackID == "" {
		statusMsg.Delete(b, nil)
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	statusMsg.Delete(b, nil)

	callbackData := fmt.Sprintf("song_%d_%s", userID, trackID)
	go downloadTrackFromCallback(b, ctx.EffectiveChat.Id, callbackData, ctx.EffectiveMessage.MessageId)

	return nil
}

func downloadTrackFromCallback(b *gotgbot.Bot, chatID int64, callbackData string, replyToMsgID int64) {
	parts := strings.Split(callbackData, "_")
	if len(parts) != 3 {
		return
	}

	trackID := parts[2]

	progressMsg, err := b.SendMessage(chatID, "🎵 <b>Downloading...</b>\n\nPlease wait...", &gotgbot.SendMessageOpts{
		ParseMode: "HTML",
		ReplyParameters: &gotgbot.ReplyParameters{
			MessageId: replyToMsgID,
		},
	})
	if err != nil {
		return
	}

	trackURL := fmt.Sprintf("https://open.spotify.com/track/%s", trackID)
	stream, err := GetTrackStream(trackURL)
	if err != nil {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
		return
	}
	if len(stream.Source) == 0 {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
		return
	}
	source := stream.Source[0]
	caption := FormatSongCaption(source.Title, source.Artist, source.Duration)
	audioOpts := &gotgbot.SendAudioOpts{
		Caption:   caption,
		Title:     source.Title,
		Performer: source.Artist,
		Duration:  int64(source.Duration),
		ParseMode: "HTML",
	}
	_, err = b.SendAudio(chatID, gotgbot.InputFileByURL(source.URL), audioOpts)
	if err != nil {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
		return
	}
	b.DeleteMessage(chatID, progressMsg.MessageId, nil)
}

func sendPlaylistPageMessage(b *gotgbot.Bot, chatID int64, info *PlaylistInfo, page int, userID int64, cacheKey string, replyToMsgID int64, chatType string) error {
	totalTracks := len(info.Tracks)
	totalPages := (totalTracks + 9) / 10
	start := page * 10
	end := start + 10
	if end > totalTracks {
		end = totalTracks
	}

	var text string
	if info.Type == "playlist" {
		text = fmt.Sprintf("📀 <b>%s</b>\n\n📊 <b>Total tracks:</b> %d\n\n<b>Page %d/%d</b>\n\n", info.Name, totalTracks, page+1, totalPages)
	} else {
		text = fmt.Sprintf("💿 <b>%s</b>\n\n📊 <b>Total tracks:</b> %d\n\n<b>Page %d/%d</b>\n\n", info.Name, totalTracks, page+1, totalPages)
	}

	keyboard := buildStatelessKeyboard(info, page, userID, cacheKey, chatType)
	replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}

	if info.Image != "" {
		if replyToMsgID != 0 {
			_, err := b.SendPhoto(chatID, gotgbot.InputFileByURL(info.Image), &gotgbot.SendPhotoOpts{
				Caption:         text,
				ParseMode:       "HTML",
				ReplyMarkup:     replyMarkup,
				ReplyParameters: &gotgbot.ReplyParameters{MessageId: replyToMsgID},
			})
			return err
		}
		_, err := b.SendPhoto(chatID, gotgbot.InputFileByURL(info.Image), &gotgbot.SendPhotoOpts{
			Caption:     text,
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		return err
	}

	if replyToMsgID != 0 {
		_, err := b.SendMessage(chatID, text, &gotgbot.SendMessageOpts{
			ParseMode:       "HTML",
			ReplyMarkup:     replyMarkup,
			ReplyParameters: &gotgbot.ReplyParameters{MessageId: replyToMsgID},
		})
		return err
	}
	_, err := b.SendMessage(chatID, text, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: replyMarkup,
	})
	return err
}

func editPlaylistPageMessage(b *gotgbot.Bot, chatID int64, messageID int64, info *PlaylistInfo, page int, userID int64, cacheKey string, chatType string) error {
	totalTracks := len(info.Tracks)
	totalPages := (totalTracks + 9) / 10
	start := page * 10
	end := start + 10
	if end > totalTracks {
		end = totalTracks
	}

	var text string
	if info.Type == "playlist" {
		text = fmt.Sprintf("📀 <b>%s</b>\n\n📊 <b>Total tracks:</b> %d\n\n<b>Page %d/%d</b>\n\n", info.Name, totalTracks, page+1, totalPages)
	} else {
		text = fmt.Sprintf("💿 <b>%s</b>\n\n📊 <b>Total tracks:</b> %d\n\n<b>Page %d/%d</b>\n\n", info.Name, totalTracks, page+1, totalPages)
	}

	keyboard := buildStatelessKeyboard(info, page, userID, cacheKey, chatType)
	replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}

	if info.Image != "" {
		_, _, err := b.EditMessageMedia(gotgbot.InputMediaPhoto{
			Media:     gotgbot.InputFileByURL(info.Image),
			Caption:   text,
			ParseMode: "HTML",
		}, &gotgbot.EditMessageMediaOpts{
			ChatId:      chatID,
			MessageId:   messageID,
			ReplyMarkup: replyMarkup,
		})
		return err
	}

	_, _, err := b.EditMessageText(text, &gotgbot.EditMessageTextOpts{
		ChatId:      chatID,
		MessageId:   messageID,
		ParseMode:   "HTML",
		ReplyMarkup: replyMarkup,
	})
	return err
}

func buildStatelessKeyboard(info *PlaylistInfo, page int, userID int64, cacheKey string, chatType string) [][]gotgbot.InlineKeyboardButton {
	totalTracks := len(info.Tracks)
	start := page * 10
	end := start + 10
	if end > totalTracks {
		end = totalTracks
	}
	keyboard := make([][]gotgbot.InlineKeyboardButton, 0)

	for i := start; i < end; i++ {
		track := info.Tracks[i]
		trackName := track.Name
		if len(trackName) > 35 {
			trackName = trackName[:32] + "..."
		}
		artistName := track.Artist
		if len(artistName) > 20 {
			artistName = artistName[:17] + "..."
		}
		buttonText := fmt.Sprintf("%d. %s - %s", i+1, trackName, artistName)
		keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
			Text:         buttonText,
			CallbackData: fmt.Sprintf("song_%d_%s", userID, track.SongID),
		}})
	}

	navRow := []gotgbot.InlineKeyboardButton{}
	if page > 0 {
		navRow = append(navRow, gotgbot.InlineKeyboardButton{Text: "◀️ Back", CallbackData: fmt.Sprintf("pg#%d#%d#%s", userID, page-1, cacheKey)})
	}
	if end < totalTracks {
		navRow = append(navRow, gotgbot.InlineKeyboardButton{Text: "Next ▶️", CallbackData: fmt.Sprintf("pg#%d#%d#%s", userID, page+1, cacheKey)})
	}
	if len(navRow) > 0 {
		keyboard = append(keyboard, navRow)
	}

	if chatType == "private" {
		keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
			Text:         "⬇️ Download All",
			CallbackData: fmt.Sprintf("dl_now#%d#%s", userID, cacheKey),
		}})
	} else {
		deepLink := fmt.Sprintf("https://t.me/%s?start=dl-%s", BotUsername, cacheKey)
		keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
			Text: "⬇️ Download All (PM)",
			Url:  deepLink,
		}})
	}

	keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
		Text:         "❌ Cancel",
		CallbackData: "cancel",
	}})
	return keyboard
}

func getOrFetchPlaylistInfo(cacheKey string) (*PlaylistInfo, error) {
	playlistCacheMu.RLock()
	cached, ok := playlistCache[cacheKey]
	playlistCacheMu.RUnlock()
	if ok && time.Now().Before(cached.ExpiresAt) {
		return cached.Info, nil
	}

	playlistID, typ, err := decodePlaylistCallback(cacheKey)
	if err != nil {
		return nil, err
	}

	info, err := GetSpotifyInfoByID(playlistID, typ)
	if err != nil {
		return nil, err
	}
	playlistCacheMu.Lock()
	playlistCache[cacheKey] = &CachedPlaylist{
		Info:      info,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	playlistCacheMu.Unlock()
	return info, nil
}

func HandlePlaylistCallback(b *gotgbot.Bot, ctx *ext.Context) error {
	query := ctx.Update.CallbackQuery
	if query == nil || query.Message == nil {
		return nil
	}
	callerID := query.From.Id
	chatID := query.Message.GetChat().Id
	messageID := query.Message.GetMessageId()
	chatType := query.Message.GetChat().Type
	data := query.Data

	if data == "cancel" {
		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Cancelled"})
		b.DeleteMessage(chatID, messageID, nil)
		return nil
	}

	if strings.HasPrefix(data, "pg#") {
		parts := strings.SplitN(data, "#", 4)
		if len(parts) != 4 {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid callback data"})
			return nil
		}

		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid user ID"})
			return nil
		}

		if userID != callerID {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "You can only control your own playlists."})
			return nil
		}

		page, err := strconv.Atoi(parts[2])
		if err != nil {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid page number"})
			return nil
		}

		cacheKey := parts[3]

		info, err := getOrFetchPlaylistInfo(cacheKey)
		if err != nil {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Something went wrong"})
			return nil
		}

		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Loading page..."})
		return editPlaylistPageMessage(b, chatID, messageID, info, page, userID, cacheKey, chatType)
	}

	if strings.HasPrefix(data, "dl_now#") {
		parts := strings.SplitN(data, "#", 3)
		if len(parts) != 3 {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid callback data"})
			return nil
		}

		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid user ID"})
			return nil
		}

		if userID != callerID {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "You can only control your own playlists."})
			return nil
		}

		cacheKey := parts[2]

		info, err := getOrFetchPlaylistInfo(cacheKey)
		if err != nil {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Something went wrong"})
			return nil
		}

		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Starting download of all tracks..."})
		setUserProcessing(userID, true)
		go downloadAllTracksToChat(b, chatID, userID, info)
		return nil
	}

	if strings.HasPrefix(data, "stop_dl#") {
		parts := strings.SplitN(data, "#", 2)
		if len(parts) != 2 {
			return nil
		}

		userID, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return nil
		}

		if userID != callerID {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "You can only stop your own downloads."})
			return nil
		}

		if exists, cancelChan := GetCancelChannel(userID); exists {
			cancelChan <- true
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Stopping download..."})
		} else {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "No active download found."})
		}
		return nil
	}

	if strings.HasPrefix(data, "retry#") {
		parts := strings.SplitN(data, "#", 2)
		if len(parts) != 2 {
			return nil
		}
		failID := parts[1]

		failedMutex.RLock()
		failed, exists := failedTracks[failID]
		failedMutex.RUnlock()

		if !exists {
			query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Download session expired."})
			return nil
		}

		query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Retrying..."})
		b.DeleteMessage(chatID, messageID, nil)
		go downloadSingleTrackWithRetry(b, chatID, failed.Track, failID, failed.RetryCount)
		return nil
	}

	return nil
}

func downloadSingleTrackByID(b *gotgbot.Bot, chatID int64, progressMsgID int64, trackID string) {
	trackURL := fmt.Sprintf("https://open.spotify.com/track/%s", trackID)
	stream, err := GetTrackStream(trackURL)
	if err != nil {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsgID})
		return
	}
	if len(stream.Source) == 0 {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsgID})
		return
	}
	source := stream.Source[0]
	caption := FormatSongCaption(source.Title, source.Artist, source.Duration)
	audioOpts := &gotgbot.SendAudioOpts{
		Caption:   caption,
		Title:     source.Title,
		Performer: source.Artist,
		Duration:  int64(source.Duration),
		ParseMode: "HTML",
	}
	_, err = b.SendAudio(chatID, gotgbot.InputFileByURL(source.URL), audioOpts)
	if err != nil {
		b.EditMessageText("❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsgID})
		return
	}
	b.DeleteMessage(chatID, progressMsgID, nil)
}

func downloadSingleTrackWithRetry(b *gotgbot.Bot, chatID int64, track PlaylistTrack, retryID string, retryCount int) {
	trackURL := fmt.Sprintf("https://open.spotify.com/track/%s", track.SongID)
	client := dlkitgo.NewClient()
	stream, err := client.Spotify.Stream(trackURL)
	if err != nil || len(stream.Source) == 0 {
		if retryCount >= 2 {
			b.SendMessage(chatID, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.SendMessageOpts{ParseMode: "HTML"})
			failedMutex.Lock()
			delete(failedTracks, retryID)
			failedMutex.Unlock()
			return
		}
		failID := GenerateRandomID()
		failedMutex.Lock()
		failedTracks[failID] = &FailedTrack{
			Track:      track,
			UserID:     0,
			ChatID:     chatID,
			RetryCount: retryCount + 1,
		}
		failedMutex.Unlock()
		keyboard := [][]gotgbot.InlineKeyboardButton{{{
			Text: "🔄 Try Again", CallbackData: fmt.Sprintf("retry#%s", failID),
		}}}
		replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}
		b.SendMessage(chatID, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		return
	}
	source := stream.Source[0]
	caption := FormatSongCaption(source.Title, source.Artist, source.Duration)
	opts := &gotgbot.SendAudioOpts{
		Caption:   caption,
		Title:     source.Title,
		Performer: source.Artist,
		Duration:  int64(source.Duration),
		ParseMode: "HTML",
	}
	_, err = b.SendAudio(chatID, gotgbot.InputFileByURL(source.URL), opts)
	if err != nil {
		if retryCount >= 2 {
			b.SendMessage(chatID, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.SendMessageOpts{ParseMode: "HTML"})
			failedMutex.Lock()
			delete(failedTracks, retryID)
			failedMutex.Unlock()
			return
		}
		failID := GenerateRandomID()
		failedMutex.Lock()
		failedTracks[failID] = &FailedTrack{
			Track:      track,
			UserID:     0,
			ChatID:     chatID,
			RetryCount: retryCount + 1,
		}
		failedMutex.Unlock()
		keyboard := [][]gotgbot.InlineKeyboardButton{{{
			Text: "🔄 Try Again", CallbackData: fmt.Sprintf("retry#%s", failID),
		}}}
		replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}
		b.SendMessage(chatID, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
		return
	}

	failedMutex.Lock()
	delete(failedTracks, retryID)
	failedMutex.Unlock()
}

func HandleDownloadAllStart(b *gotgbot.Bot, ctx *ext.Context, encodedID string) error {
	chat := ctx.EffectiveChat
	if chat.Type != "private" {
		ctx.EffectiveMessage.Reply(b, "Please use this command in private chat with me.", nil)
		return nil
	}
	userID := ctx.EffectiveUser.Id

	playlistID, typ, err := decodePlaylistCallback(encodedID)
	if err != nil {
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	cacheKey := EncodePlaylistCallback(playlistID, typ)
	info, err := getOrFetchPlaylistInfo(cacheKey)
	if err != nil {
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	playlistCacheMu.Lock()
	if _, exists := playlistCache[cacheKey]; !exists {
		playlistCache[cacheKey] = &CachedPlaylist{
			Info:      info,
			ExpiresAt: time.Now().Add(1 * time.Hour),
		}
	}
	playlistCacheMu.Unlock()

	return sendPlaylistPageMessage(b, chat.Id, info, 0, userID, cacheKey, ctx.EffectiveMessage.MessageId, "private")
}

func downloadAllTracksToChat(b *gotgbot.Bot, chatID int64, userID int64, info *PlaylistInfo) {
	defer setUserProcessing(userID, false)
	total := len(info.Tracks)
	success := 0
	fail := 0
	failedList := make([]PlaylistTrack, 0)

	cancelChan := make(chan bool)
	SetCancelChannel(userID, cancelChan)
	defer func() { RemoveCancelChannel(userID) }()

	stopButton := gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
			{Text: "🛑 Stop Download", CallbackData: fmt.Sprintf("stop_dl#%d", userID)},
		}},
	}
	statusMsg, err := b.SendMessage(chatID, fmt.Sprintf("⬇️ Downloading %d tracks...\n\nProgress: 0/%d\n✅ Success: 0\n❌ Failed: 0", total, total), &gotgbot.SendMessageOpts{
		ReplyMarkup: stopButton,
	})
	if err != nil {
		return
	}

	for i, track := range info.Tracks {
		select {
		case <-cancelChan:
			b.EditMessageText("⏹️ Download cancelled by user.", &gotgbot.EditMessageTextOpts{
				ChatId:    chatID,
				MessageId: statusMsg.MessageId,
				ParseMode: "HTML",
			})
			return
		default:
		}

		client := dlkitgo.NewClient()
		stream, err := client.Spotify.Stream(track.URL)
		if err != nil || len(stream.Source) == 0 {
			fail++
			failedList = append(failedList, track)
		} else {
			source := stream.Source[0]
			caption := FormatSongCaption(source.Title, source.Artist, source.Duration)
			opts := &gotgbot.SendAudioOpts{
				Caption:   caption,
				Title:     source.Title,
				Performer: source.Artist,
				Duration:  int64(source.Duration),
				ParseMode: "HTML",
			}
			_, err = b.SendAudio(chatID, gotgbot.InputFileByURL(source.URL), opts)
			if err != nil {
				fail++
				failedList = append(failedList, track)
			} else {
				success++
			}
		}

		progressText := fmt.Sprintf("⬇️ Downloading %d tracks...\n\nProgress: %d/%d\n✅ Success: %d\n❌ Failed: %d", total, i+1, total, success, fail)
		b.EditMessageText(progressText, &gotgbot.EditMessageTextOpts{
			ChatId:      chatID,
			MessageId:   statusMsg.MessageId,
			ParseMode:   "HTML",
			ReplyMarkup: stopButton,
		})
		if i < total-1 {
			time.Sleep(900 * time.Millisecond)
		}
	}

	finalText := fmt.Sprintf("✅ <b>Playlist Download Complete!</b>\n\n📊 Total: %d\n✅ Success: %d\n❌ Failed: %d", total, success, fail)
	b.EditMessageText(finalText, &gotgbot.EditMessageTextOpts{
		ChatId:    chatID,
		MessageId: statusMsg.MessageId,
		ParseMode: "HTML",
	})

	for _, failed := range failedList {
		failID := GenerateRandomID()
		failedMutex.Lock()
		failedTracks[failID] = &FailedTrack{
			Track:      failed,
			UserID:     userID,
			ChatID:     chatID,
			RetryCount: 0,
		}
		failedMutex.Unlock()
		keyboard := [][]gotgbot.InlineKeyboardButton{{{
			Text: "🔄 Try Again", CallbackData: fmt.Sprintf("retry#%s", failID),
		}}}
		replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}
		b.SendMessage(chatID, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.SendMessageOpts{
			ParseMode:   "HTML",
			ReplyMarkup: replyMarkup,
		})
	}
}
