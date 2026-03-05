package bot

import (
    "log"
    "context"

    "github.com/Beesonn/MediaSaveBot/database"
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func Start(b *gotgbot.Bot, ctx *ext.Context) error {
    user := ctx.EffectiveUser
    chat := ctx.EffectiveChat
    
    dbUser := &database.User{
        UserID:    user.Id,
        FirstName: user.FirstName,
        LastName:  user.LastName,
        Username:  user.Username,
        ChatID:    chat.Id,
    }
    
    if err := database.SaveUser(context.Background(), dbUser); err != nil {
        log.Printf("Error saving user: %v\n", err)
    }

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
        {Text: "📢 Support Channel", Url: "https://t.me/BeesonsBots"},
    }}

    replyMarkup := &gotgbot.InlineKeyboardMarkup{
        InlineKeyboard: keyboard,
    }

    _, err := ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
        ReplyMarkup: replyMarkup,
    })
    
    return err
}
