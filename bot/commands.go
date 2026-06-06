package bot

import (
    "context"
    "fmt"
    "os"
    "regexp"
    "strconv"
    "strings"
    "time"

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

    if len(command) == 2 && command[1] == "donate" {
        return Donate(b, ctx)
    }

    if database.IsMongoAvailable() {
        if utils.BotUsername != b.User.Username {
            go database.SaveCloneBot(b.User.Id, user.Id, b.User.Username, "")
            go database.SaveCloneBotUser(b.User.Id, user.Id, user.FirstName)
        } else {
            go database.SaveUser(context.Background(), user.FirstName, user.Id)
        }
    }

    if chat.Type != "private" {
        text := "I'm a media download bot! Send me any link from Spotify, YouTube, Instagram, or Pinterest and I'll download it for you."
        _, err := ctx.EffectiveMessage.Reply(b, text, nil)
        return err
    }

    var text string
    if utils.BotUsername != b.User.Username {
        text = fmt.Sprintf(`Hello! 👋

I'm @%s! 📥

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
Usage: <code>/donate 100</code>`, b.User.Username)
    } else {
        text = `Hello! 👋

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
    }

    if database.IsMongoAvailable() && isAdmin(user.Id) && utils.BotUsername == b.User.Username {
        text += `

/stats - Bot statistics (Admin only)
/broadcast - Broadcast message (Admin only)
/allbroadcast - Broadcast to all bot users (Admin only)
/restartallbots - Restart all clone bots (Admin only)`
    }

    keyboard := [][]gotgbot.InlineKeyboardButton{
        {
            {Text: "🔍 Inline Search", SwitchInlineQueryCurrentChat: &[]string{""}[0]},
        },
        {
            {Text: "🤖 Create your own bot", CallbackData: "create_bot"},
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

    if chat.Type != "private" {
        _, err := ctx.EffectiveMessage.Reply(b, "Please use this command in private chat with me.", nil)
        return err
    }

    if utils.BotUsername != b.User.Username {
        mainBotUsername := utils.BotUsername
        text := fmt.Sprintf("💝 Support our project!\n\nClick the button below to donate to the main bot @%s", mainBotUsername)
        
        keyboard := [][]gotgbot.InlineKeyboardButton{
            {{
                Text: "💝 Donate Stars",
                Url: fmt.Sprintf("https://t.me/%s?start=donate", mainBotUsername),
            }},
        }
        
        replyMarkup := &gotgbot.InlineKeyboardMarkup{
            InlineKeyboard: keyboard,
        }
        
        _, err := ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
            ReplyMarkup: replyMarkup,
        })
        return err
    }

    user := ctx.EffectiveUser
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

func HandleCreateBotCallback(b *gotgbot.Bot, ctx *ext.Context) error {
    query := ctx.Update.CallbackQuery
    if query == nil {
        return nil
    }

    if query.Data != "create_bot" {
        return nil
    }

    query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{})

    text := `🤖 <b>Create your own bot like me!</b>

Follow these steps to create your own bot:

<b>Step 1:</b>
Create a bot from @BotFather
• Send /newbot to @BotFather
• Give it a good name
• Then provide a username (must end with 'bot')

<b>Step 2:</b>
Copy the bot token from @BotFather and send it to me
Example: <code>1234567890:ABCdefGHIjklMNOpqrsTUVwxyz-12345</code>

<b>Step 3:</b>
I'll create a bot like me for you automatically!

<b>Step 4:</b>
Enjoy your own media download bot! 🎉

If you have any doubts, don't hesitate to ask here: @XBOTSUPPORTS`

    _, err := b.SendMessage(query.Message.GetChat().Id, text, &gotgbot.SendMessageOpts{
        ParseMode: "HTML",
    })
    return err
}

func HandleMessage(b *gotgbot.Bot, ctx *ext.Context) error {
    if ctx.EffectiveMessage.Text == "" {
        return nil
    }

    text := ctx.EffectiveMessage.Text
    chat := ctx.EffectiveChat

    botTokenRegex := regexp.MustCompile(`\b[0-9]{9,11}:[a-zA-Z0-9_\-]{35}\b`)
    if botTokenRegex.MatchString(text) {
        webhookURL := os.Getenv("WEBHOOK_URL")
        if webhookURL == "" {
            return nil
        }
        
        botToken := botTokenRegex.FindString(text)
        
        parts := strings.Split(botToken, ":")
        if len(parts) != 2 {
            return nil
        }
        
        botID, err := strconv.ParseInt(parts[0], 10, 64)
        if err != nil {
            return nil
        }
        
        existingBot, _ := database.GetCloneBotByID(botID)
        if existingBot != nil {
            ctx.EffectiveMessage.Reply(b, "⚠️ This bot has already been cloned. Each bot can only be cloned once.\n\nSupport: @XBOTSUPPORTS", nil)
            return nil
        }
        
        statusMsg, err := ctx.EffectiveMessage.Reply(b, "🔄 Creating your bot... Please wait.", nil)
        if err != nil {
            return err
        }

        cloneBot, err := gotgbot.NewBot(botToken, &gotgbot.BotOpts{
            RequestOpts: &gotgbot.RequestOpts{
                Timeout: time.Minute * 50,
            },
        })
        if err != nil {
            statusMsg.Delete(b, nil)
            ctx.EffectiveMessage.Reply(b, "❌ Invalid bot token. Please make sure you sent the correct token from @BotFather.\n\nSupport: @XBOTSUPPORTS", nil)
            return nil
        }

        webhookURL = strings.TrimSuffix(webhookURL, "/")
        webhookEndpoint := fmt.Sprintf("%s/webhook/%s", webhookURL, botToken)
        
        cloneBot.DeleteWebhook(&gotgbot.DeleteWebhookOpts{
            DropPendingUpdates: true,
        })
        
        _, err = cloneBot.SetWebhook(webhookEndpoint, &gotgbot.SetWebhookOpts{
            DropPendingUpdates: true,
        })
        if err != nil {
            statusMsg.Delete(b, nil)
            ctx.EffectiveMessage.Reply(b, "❌ Failed to create bot. Your bot token might be invalid or expired.\n\nPlease check your token and try again.\n\nSupport: @XBOTSUPPORTS", nil)
            return nil
        }

        go database.SaveCloneBot(cloneBot.User.Id, ctx.EffectiveUser.Id, cloneBot.User.Username, botToken)

        successText := fmt.Sprintf("✅ Successfully created bot @%s\n\nYou can now use this bot to download media from Spotify, YouTube, Instagram, and Pinterest!\n\nEnjoy! 🎉", cloneBot.User.Username)
        
        statusMsg.Delete(b, nil)
        _, err = ctx.EffectiveMessage.Reply(b, successText, nil)
        return err
    }

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
