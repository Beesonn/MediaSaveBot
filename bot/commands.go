package bot

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Beesonn/MediaSaveBot/database"
	"github.com/Beesonn/MediaSaveBot/utils"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func Start(b *gotgbot.Bot, ctx *ext.Context) error {
	user := ctx.EffectiveUser
	chat := ctx.EffectiveChat
	message := ctx.EffectiveMessage

	command := strings.Split(strings.Replace(message.Text, "/", "", 1), " ")

	if len(command) == 2 && strings.HasPrefix(command[1], "dl_") {
		token := strings.TrimPrefix(command[1], "dl_")
		return utils.HandleDownloadAllStart(b, ctx, token)
	}

	if database.IsMongoAvailable() {
		go database.SaveUser(context.Background(), user.FirstName, user.Id)
	}

	if chat.Type != "private" {
		text := "Hey, I'm ready to download media just send me link"
		_, err := ctx.EffectiveMessage.Reply(b, text, nil)
		return err
	}

	text := `Hello! 👋

I'm a Media Save Download bot! 📥

I can download anything from:
🎧 Spotify
📸 Pinterest
📷 Instagram
🎬 YouTube
and more!

📝 <b>Commands:</b>

/song - Search and download songs
Usage: <code>/song song name</code>

/donate - Support the bot with Telegram Stars
Usage: <code>/donate 100</code>`

	if database.IsMongoAvailable() && isAdmin(user.Id) {
		text += `

/stats - Bot statistics (Admin only)
/broadcast - Broadcast message (Admin only)`
	}

	keyboard := [][]gotgbot.InlineKeyboardButton{
		{
			{Text: "🔍 Inline Search", SwitchInlineQueryCurrentChat: &[]string{""}[0]},
		},
		{
			{Text: "👥 Support Group", Url: "https://t.me/XBOTSUPPORTS"},
			{Text: "📢 Update Channel", Url: "https://t.me/BeesonsBots"},
		},
		{
			{Text: "💻 Source Code", Url: "https://github.com/Beesonn/MediaSaveBot"},
		},
	}

	replyMarkup := &gotgbot.InlineKeyboardMarkup{
		InlineKeyboard: keyboard,
	}

	_, err := ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
		ParseMode:   "HTML",
		ReplyMarkup: replyMarkup,
	})

	return err
}

func Donate(b *gotgbot.Bot, ctx *ext.Context) error {
	chat := ctx.EffectiveChat
	message := ctx.EffectiveMessage
	user := ctx.EffectiveUser

	if chat.Type != "private" {
		_, err := ctx.EffectiveMessage.Reply(b, "Please use this command in private chat with me.", nil)
		return err
	}

	command := strings.Split(strings.TrimSpace(message.Text), " ")

	var amount int64 = 100

	if len(command) == 2 {
		amountStr := command[1]
		parsedAmount, err := strconv.ParseInt(amountStr, 10, 64)
		if err == nil && parsedAmount > 0 {
			amount = parsedAmount
		}
	}

	if amount < 1 {
		amount = 100
	}

	title := fmt.Sprintf("Donate %d Stars", amount)
	description := "Support our bot! Donations help with server costs, development, and maintenance. 100% free & open source. Thank you! 🙏"

	payload := fmt.Sprintf("donate_%d_%d", user.Id, amount)
	currency := "XTR"
	prices := []gotgbot.LabeledPrice{
		{Label: fmt.Sprintf("%d Stars", amount), Amount: amount},
	}

	_, err := ctx.EffectiveMessage.ReplyInvoice(b, title, description, payload, currency, prices, nil)
	return err
}

func HandleMessage(b *gotgbot.Bot, ctx *ext.Context) error {
	if ctx.EffectiveMessage.Text == "" {
		return nil
	}

	text := ctx.EffectiveMessage.Text
	chat := ctx.EffectiveChat

	instagramRegex := regexp.MustCompile(`(https?://)?(www\.)?(instagram\.com|instagr\.am)/.+`)
	if instagramRegex.MatchString(text) {
		return utils.HandleInstagram(b, ctx)
	}

	pinterestRegex := regexp.MustCompile(`(https?://)?(www\.)?(pinterest\.com|pin\.it)/.+`)
	if pinterestRegex.MatchString(text) {
		return utils.HandlePinterest(b, ctx)
	}

	spotifyRegex := regexp.MustCompile(`(https?://)?(www\.)?(open\.spotify\.com)/.+`)
	if spotifyRegex.MatchString(text) {
		return utils.HandleSpotify(b, ctx)
	}

	youtubeRegex := regexp.MustCompile(`(https?://)?(www\.)?(youtu\.be|youtube\.com)/.+`)
	if youtubeRegex.MatchString(text) {
		return utils.HandleYoutube(b, ctx)
	}

	if chat.Type == "private" {
		return GetSearchResults(b, ctx, text)
	}

	return nil
}
