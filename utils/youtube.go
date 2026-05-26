package utils

import (
    "fmt"
    "io"
    "net/http"
    "net/url"
    "os"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/Beesonn/dlkitgo"
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var (
    youtubeCache   = make(map[string]*YoutubeCache)
    youtubeCacheMu = &sync.RWMutex{}
    httpClient     = &http.Client{}
)

type YoutubeVideo struct {
    Name        string
    ChannelName string
    Duration    int
    URL         string
    VideoID     string
}

type YoutubeInfo struct {
    Type         string
    ID           string
    Name         string
    Image        string
    TotalVideos  int
    Videos       []YoutubeVideo
    PlaylistID   string
}

type YoutubeCache struct {
    Info      *YoutubeInfo
    MessageID int64
    ExpiresAt time.Time
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
        `playlist\?list=([a-zA-Z0-9_-]+)`,
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
        if list := parsed.Query().Get("list"); list != "" {
            return list
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
        PlaylistID:  info.ID,
    }

    if info.Type == "playlist" {
        for _, item := range info.Playlist {
            youtubeInfo.Videos = append(youtubeInfo.Videos, YoutubeVideo{
                Name:        EscapeHTML(item.Name),
                ChannelName: EscapeHTML(item.ChannelName),
                Duration:    item.Duration,
                URL:         item.URL,
                VideoID:     ExtractYoutubeID(item.URL),
            })
        }
        youtubeInfo.TotalVideos = len(youtubeInfo.Videos)
    } else if info.Type == "video" || info.Type == "shorts" {
        if len(info.Videos) > 0 {
            youtubeInfo.TotalVideos = 1
            youtubeInfo.Videos = append(youtubeInfo.Videos, YoutubeVideo{
                Name:        EscapeHTML(info.Name),
                ChannelName: EscapeHTML(info.Videos[0].ChannelName),
                Duration:    info.Videos[0].Duration,
                URL:         rawURL,
                VideoID:     info.ID,
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
    userID := ctx.EffectiveMessage.From.Id
    chatID := ctx.EffectiveChat.Id
    chatType := ctx.EffectiveChat.Type

    if isUserProcessing(userID) {
        _, err := ctx.EffectiveMessage.Reply(b,
            " You already have an ongoing task. Please wait for it to finish before sending another request.",
            nil)
        return err
    }

    rawURL := ctx.EffectiveMessage.Text
    if idx := strings.Index(rawURL, "&si="); idx != -1 {
        rawURL = rawURL[:idx]
    }

    videoID := ExtractYoutubeID(rawURL)
    if videoID == "" {
        ctx.EffectiveMessage.Reply(b, " Could not extract YouTube ID. Please check the link.", nil)
        return nil
    }

    info, err := GetYoutubeInfo(rawURL)
    if err != nil {
        ctx.EffectiveMessage.Reply(b, " Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
        return nil
    }

    if info.Type == "playlist" {
        if info.TotalVideos == 0 {
            ctx.EffectiveMessage.Reply(b, " No videos found in this playlist.", nil)
            return nil
        }
        if info.TotalVideos == 1 {
            return handleYoutubeVideo(b, ctx, info.Videos[0].URL, userID, chatID)
        }
        return handleYoutubePlaylist(b, ctx, info, userID, chatID, chatType)
    }

    return handleYoutubeVideo(b, ctx, rawURL, userID, chatID)
}

func handleYoutubePlaylist(b *gotgbot.Bot, ctx *ext.Context, info *YoutubeInfo, userID, chatID int64, chatType string) error {
    cacheKey := fmt.Sprintf("yt_%s", info.PlaylistID)
    youtubeCacheMu.Lock()
    youtubeCache[cacheKey] = &YoutubeCache{
        Info:      info,
        ExpiresAt: time.Now().Add(1 * time.Hour),
    }
    youtubeCacheMu.Unlock()

    return sendYoutubePlaylistPage(b, chatID, info, 0, userID, cacheKey, 0)
}

func sendYoutubePlaylistPage(b *gotgbot.Bot, chatID int64, info *YoutubeInfo, page int, userID int64, cacheKey string, botMsgID int64) error {
    totalVideos := info.TotalVideos
    if totalVideos == 0 {
        return nil
    }
    totalPages := (totalVideos + 9) / 10
    start := page * 10
    end := start + 10
    if end > totalVideos {
        end = totalVideos
    }

    text := fmt.Sprintf(" <b>%s</b>\n\n <b>Total videos:</b> %d\n\n<b>Page %d/%d</b>\n\n", info.Name, totalVideos, page+1, totalPages)

    keyboard := make([][]gotgbot.InlineKeyboardButton, 0)

    for i := start; i < end; i++ {
        video := info.Videos[i]
        videoName := video.Name
        if len(videoName) > 35 {
            videoName = videoName[:32] + "..."
        }
        durationMin := video.Duration / 60
        durationSec := video.Duration % 60
        buttonText := fmt.Sprintf("%d.  %s - %s (%d:%02d)", i+1, videoName, video.ChannelName, durationMin, durationSec)
        keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
            Text:         buttonText,
            CallbackData: fmt.Sprintf("yt_tr_%d_%s_%d", userID, cacheKey, i),
        }})
    }

    navRow := []gotgbot.InlineKeyboardButton{}
    if page > 0 {
        navRow = append(navRow, gotgbot.InlineKeyboardButton{Text: " Back", CallbackData: fmt.Sprintf("yt_pg_%d_%s_%d", userID, cacheKey, page-1)})
    }
    if end < totalVideos {
        navRow = append(navRow, gotgbot.InlineKeyboardButton{Text: "Next ", CallbackData: fmt.Sprintf("yt_pg_%d_%s_%d", userID, cacheKey, page+1)})
    }
    if len(navRow) > 0 {
        keyboard = append(keyboard, navRow)
    }

    keyboard = append(keyboard, []gotgbot.InlineKeyboardButton{{
        Text:         " Cancel",
        CallbackData: "cancel",
    }})

    replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}

    if botMsgID == 0 {
        sentMsg, err := b.SendMessage(chatID, text, &gotgbot.SendMessageOpts{
            ParseMode:   "HTML",
            ReplyMarkup: replyMarkup,
        })
        if err != nil {
            return err
        }
        youtubeCacheMu.Lock()
        if cached, exists := youtubeCache[cacheKey]; exists {
            cached.MessageID = sentMsg.MessageId
        }
        youtubeCacheMu.Unlock()
    } else {
        _, _, err := b.EditMessageText(text, &gotgbot.EditMessageTextOpts{
            ChatId:      chatID,
            MessageId:   botMsgID,
            ParseMode:   "HTML",
            ReplyMarkup: replyMarkup,
        })
        if err != nil {
            return err
        }
    }
    return nil
}

func handleYoutubeVideo(b *gotgbot.Bot, ctx *ext.Context, url string, userID, chatID int64) error {
    statusMsg, err := ctx.EffectiveMessage.Reply(b, " Processing YouTube video...", nil)
    if err != nil {
        return err
    }

    stream, err := GetYoutubeStream(url)
    if err != nil {
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, " Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
        return nil
    }

    statusMsg.Delete(b, nil)

    durationMin := stream.Duration / 60
    durationSec := stream.Duration % 60

    text := fmt.Sprintf(" <b>%s</b>\n\n <b>Duration:</b> %d:%02d\n\n <b>Choose download format:</b>", stream.Title, durationMin, durationSec)

    keyboard := [][]gotgbot.InlineKeyboardButton{
        {
            {Text: " Video (MP4)", CallbackData: fmt.Sprintf("yt_video_%d_%s", userID, ExtractYoutubeID(url))},
            {Text: " Audio (MP3)", CallbackData: fmt.Sprintf("yt_audio_%d_%s", userID, ExtractYoutubeID(url))},
        },
        {
            {Text: " Cancel", CallbackData: "cancel"},
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
    data := query.Data
    callerID := query.From.Id
    chatID := query.Message.GetChat().Id
    messageID := query.Message.GetMessageId()

    if data == "cancel" {
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Cancelled"})
        b.DeleteMessage(chatID, messageID, nil)
        return nil
    }

    parts := strings.Split(data, "_")
    if len(parts) < 4 {
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid callback data"})
        return nil
    }

    if parts[0] != "yt" {
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid prefix"})
        return nil
    }

    action := parts[1]
    userID, err := strconv.ParseInt(parts[2], 10, 64)
    if err != nil {
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid user"})
        return nil
    }

    if userID != callerID {
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "You can only control your own playlists."})
        return nil
    }

    if action == "video" || action == "audio" {
        videoID := parts[3]
        if action == "video" {
            query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Downloading video...", ShowAlert: false})
            go downloadYoutubeVideo(b, chatID, videoID)
        } else {
            query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Downloading audio...", ShowAlert: false})
            go downloadYoutubeAudio(b, chatID, videoID)
        }
        return nil
    }

    if len(parts) < 5 {
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Incomplete callback data"})
        return nil
    }

    cacheKeyParts := parts[3 : len(parts)-1]
    cacheKey := strings.Join(cacheKeyParts, "_")
    lastPart := parts[len(parts)-1]

    youtubeCacheMu.RLock()
    cached, exists := youtubeCache[cacheKey]
    youtubeCacheMu.RUnlock()
    if !exists || time.Now().After(cached.ExpiresAt) {
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Session expired. Please send the playlist link again."})
        b.DeleteMessage(chatID, messageID, nil)
        return nil
    }

    switch action {
    case "pg":
        page, err := strconv.Atoi(lastPart)
        if err != nil {
            query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid page number"})
            return nil
        }
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Loading page..."})
        return sendYoutubePlaylistPage(b, chatID, cached.Info, page, userID, cacheKey, cached.MessageID)

    case "tr":
        idx, err := strconv.Atoi(lastPart)
        if err != nil {
            query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Invalid track index"})
            return nil
        }
        if idx < 0 || idx >= len(cached.Info.Videos) {
            query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Video not found."})
            return nil
        }
        video := cached.Info.Videos[idx]
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Processing...", ShowAlert: false})
        b.DeleteMessage(chatID, messageID, nil)

        durationMin := video.Duration / 60
        durationSec := video.Duration % 60
        text := fmt.Sprintf(" <b>%s</b>\n\n <b>Channel:</b> %s\n <b>Duration:</b> %d:%02d\n\n <b>Choose download format:</b>", video.Name, video.ChannelName, durationMin, durationSec)

        keyboard := [][]gotgbot.InlineKeyboardButton{
            {
                {Text: " Video (MP4)", CallbackData: fmt.Sprintf("yt_video_%d_%s", userID, video.VideoID)},
                {Text: " Audio (MP3)", CallbackData: fmt.Sprintf("yt_audio_%d_%s", userID, video.VideoID)},
            },
            {
                {Text: " Cancel", CallbackData: "cancel"},
            },
        }
        replyMarkup := gotgbot.InlineKeyboardMarkup{InlineKeyboard: keyboard}
        _, err = b.SendMessage(chatID, text, &gotgbot.SendMessageOpts{
            ParseMode:   "HTML",
            ReplyMarkup: replyMarkup,
        })
        return err

    default:
        query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{Text: "Unknown action"})
        return nil
    }
}

func downloadYoutubeVideo(b *gotgbot.Bot, chatID int64, videoID string) {
    url := fmt.Sprintf("https://youtu.be/%s", videoID)
    stream, err := GetYoutubeStream(url)
    if err != nil {
        b.SendMessage(chatID, " Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
        return
    }

    if stream.VideoURL == "" {
        b.SendMessage(chatID, " No video stream available for this video.", nil)
        return
    }

    progressMsg, err := b.SendMessage(chatID, " Downloading video...\nPlease wait...", &gotgbot.SendMessageOpts{ParseMode: "HTML"})
    if err != nil {
        return
    }

    tempFilePath, err := DownloadFileToTemp(stream.VideoURL)
    if err != nil {
        b.EditMessageText(" Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
        return
    }
    defer os.Remove(tempFilePath)

    durationMin := stream.Duration / 60
    durationSec := stream.Duration % 60
    caption := fmt.Sprintf(" <b>%s</b>\n\n <b>Duration:</b> %d:%02d", stream.Title, durationMin, durationSec)

    videoFile, err := os.Open(tempFilePath)
    if err != nil {
        b.EditMessageText(" Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
        return
    }
    defer videoFile.Close()

    videoOpts := &gotgbot.SendVideoOpts{
        Caption:   caption,
        ParseMode: "HTML",
    }

    _, err = b.SendVideo(chatID, gotgbot.InputFileByReader(tempFilePath, videoFile), videoOpts)
    if err != nil {
        b.EditMessageText(" Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
        return
    }

    b.DeleteMessage(chatID, progressMsg.MessageId, nil)
}

func downloadYoutubeAudio(b *gotgbot.Bot, chatID int64, videoID string) {
    url := fmt.Sprintf("https://youtu.be/%s", videoID)
    stream, err := GetYoutubeStream(url)
    if err != nil {
        b.SendMessage(chatID, " Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
        return
    }

    if stream.AudioURL == "" {
        b.SendMessage(chatID, " No audio stream available for this video.", nil)
        return
    }

    progressMsg, err := b.SendMessage(chatID, " Downloading audio...\nPlease wait...", &gotgbot.SendMessageOpts{ParseMode: "HTML"})
    if err != nil {
        return
    }

    tempFilePath, err := DownloadFileToTemp(stream.AudioURL)
    if err != nil {
        b.EditMessageText(" Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
        return
    }
    defer os.Remove(tempFilePath)

    durationMin := stream.Duration / 60
    durationSec := stream.Duration % 60
    caption := fmt.Sprintf(" <b>%s</b>\n\n <b>Duration:</b> %d:%02d", stream.Title, durationMin, durationSec)

    audioFile, err := os.Open(tempFilePath)
    if err != nil {
        b.EditMessageText(" Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
        return
    }
    defer audioFile.Close()

    audioOpts := &gotgbot.SendAudioOpts{
        Caption:   caption,
        Title:     stream.Title,
        Performer: "YouTube",
        Duration:  int64(stream.Duration),
        ParseMode: "HTML",
    }

    _, err = b.SendAudio(chatID, gotgbot.InputFileByReader(tempFilePath, audioFile), audioOpts)
    if err != nil {
        b.EditMessageText(" Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", &gotgbot.EditMessageTextOpts{ChatId: chatID, MessageId: progressMsg.MessageId})
        return
    }

    b.DeleteMessage(chatID, progressMsg.MessageId, nil)
}