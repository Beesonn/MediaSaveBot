package bot

import (
    "context"
    "regexp"

    "github.com/Beesonn/MediaSaveBot/database"
    "github.com/Beesonn/MediaSaveBot/utils"
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func Start(b *gotgbot.Bot, ctx *ext.Context) error {
    user := ctx.EffectiveUser
    go database.SaveUser(context.Background(), user.FirstName, user.Id)
    
    text := "Hello! 👋\n\n" +
        "I'm a Media Save Download bot! 📥\n\n" +
        "I can download anything from:\n" +
        "🎵 YouTube\n" +
        "🎧 Spotify\n" +
        "📸 Pinterest\n" +
        "📷 Instagram\n" +
        "and more!\n\n" +
        "Just send me a link and I'll download it for you!"

    keyboard := [][]gotgbot.InlineKeyboardButton{{
        {Text: "👥 Support Group", Url: "https://t.me/XBOTSUPPORTS"},
        {Text: "📢 Update Channel", Url: "https://t.me/BeesonsBots"},
    }}

    replyMarkup := &gotgbot.InlineKeyboardMarkup{
        InlineKeyboard: keyboard,
    }

    _, err := ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
        ReplyMarkup: replyMarkup,
    })
    
    return err
}

func HandleMessage(b *gotgbot.Bot, ctx *ext.Context) error {
    if ctx.EffectiveMessage.Text == "" {
        return nil
    }

    text := ctx.EffectiveMessage.Text

    instagramRegex := regexp.MustCompile(`(https?://)?(www\.)?(instagram\.com|instagr\.am)/.+`)
    if instagramRegex.MatchString(text) {
        return utils.HandleInstagram(b, ctx)
    }

    pinterestRegex := regexp.MustCompile(`(https?://)?(www\.)?(pinterest\.com|pin\.it)/.+`)
    if pinterestRegex.MatchString(text) {
        return utils.HandlePinterest(b, ctx)
    }

    return nil
}
