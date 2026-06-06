package bot

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "html"
    "log"
    "os"
    "os/exec"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "time"

    "github.com/Beesonn/MediaSaveBot/database"
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
)

var (
    broadcastActive = false
    broadcastMu     sync.Mutex
    broadcastStop   = make(chan bool)
    adminIDs        map[int64]bool
)

func init() {
    adminIDs = make(map[int64]bool)
    adminEnv := os.Getenv("ADMIN")
    if adminEnv != "" {
        adminList := strings.Split(adminEnv, " ")
        for _, idStr := range adminList {
            id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
            if err == nil && id > 0 {
                adminIDs[id] = true
            }
        }
    }
    log.Printf("Loaded %d admin IDs", len(adminIDs))
}

func isAdmin(userID int64) bool {
    return adminIDs[userID]
}

func Stats(b *gotgbot.Bot, ctx *ext.Context) error {
    if !isAdmin(ctx.EffectiveUser.Id) {
        return nil
    }

    if !database.IsMongoAvailable() {
        _, err := ctx.EffectiveMessage.Reply(b, "❌ MongoDB is not configured. Please set MONGODB_URI environment variable to use this command.", nil)
        return err
    }

    userCount, err := database.GetUserCount(context.Background())
    if err != nil {
        _, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error getting stats: %v", err), nil)
        return err
    }

    botCount, err := database.GetCloneBotCount(context.Background())
    if err != nil {
        _, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error getting bot stats: %v", err), nil)
        return err
    }

    allBotUsersCount := database.GetAllCloneBotsUsersCount()

    text := fmt.Sprintf(
        "📊 <b>Bot Statistics</b>\n\n"+
            "👥 <b>Total Users:</b> %d\n"+
            "🤖 <b>Total Bots:</b> %d\n"+
            "📈 <b>Total All Bot Users:</b> %d",
        userCount, botCount, allBotUsersCount,
    )

    _, err = ctx.EffectiveMessage.Reply(b, text, &gotgbot.SendMessageOpts{
        ParseMode: "HTML",
    })

    return err
}

func Broadcast(b *gotgbot.Bot, ctx *ext.Context) error {
    if !isAdmin(ctx.EffectiveUser.Id) {
        return nil
    }

    if !database.IsMongoAvailable() {
        _, err := ctx.EffectiveMessage.Reply(b, "❌ MongoDB is not configured. Please set MONGODB_URI environment variable to use this command.", nil)
        return err
    }

    broadcastMu.Lock()
    if broadcastActive {
        broadcastMu.Unlock()
        _, err := ctx.EffectiveMessage.Reply(b, "⚠️ A broadcast is already in progress. Use stop button to stop it.", nil)
        return err
    }
    broadcastActive = true
    broadcastMu.Unlock()

    if ctx.EffectiveMessage.ReplyToMessage == nil {
        broadcastMu.Lock()
        broadcastActive = false
        broadcastMu.Unlock()
        _, err := ctx.EffectiveMessage.Reply(b, "❌ Please reply to a message to broadcast it.", nil)
        return err
    }

    statusMsg, err := ctx.EffectiveMessage.Reply(b, "📢 Broadcast started...\n\nProgress: 0%", nil)
    if err != nil {
        broadcastMu.Lock()
        broadcastActive = false
        broadcastMu.Unlock()
        return err
    }

    users, err := database.GetAllUsers(context.Background())
    if err != nil {
        broadcastMu.Lock()
        broadcastActive = false
        broadcastMu.Unlock()
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error getting users: %v", err), nil)
        return err
    }

    if len(users) == 0 {
        broadcastMu.Lock()
        broadcastActive = false
        broadcastMu.Unlock()
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, "❌ No users found in database.", nil)
        return nil
    }

    total := len(users)
    success := 0
    failed := 0

    stopButton := gotgbot.InlineKeyboardMarkup{
        InlineKeyboard: [][]gotgbot.InlineKeyboardButton{{
            {Text: "🛑 Stop Broadcast", CallbackData: "stop_broadcast"},
        }},
    }

    for i, user := range users {
        select {
        case <-broadcastStop:
            broadcastMu.Lock()
            broadcastActive = false
            broadcastMu.Unlock()
            statusMsg.Delete(b, nil)
            finalText := fmt.Sprintf(
                "⏸️ Broadcast Stopped\n\n"+
                    "Total Users: %d\n"+
                    "✅ Success: %d\n"+
                    "❌ Failed: %d\n"+
                    "Stopped at: %d/%d",
                total, success, failed, i, total,
            )
            ctx.EffectiveMessage.Reply(b, finalText, nil)
            return nil

        default:
            _, err := b.CopyMessage(user.UserID, ctx.EffectiveChat.Id, ctx.EffectiveMessage.ReplyToMessage.MessageId, nil)

            if err != nil {
                failed++
                log.Printf("Failed to send to user %d: %v", user.UserID, err)
            } else {
                success++
            }

            if i%5 == 0 || i == total-1 {
                percentage := (i + 1) * 100 / total
                progressText := fmt.Sprintf(
                    "📢 Broadcast in progress...\n\n"+
                        "Progress: %d%% (%d/%d)\n"+
                        "✅ Success: %d\n"+
                        "❌ Failed: %d",
                    percentage, i+1, total, success, failed,
                )

                _, _, err := statusMsg.EditText(b, progressText, &gotgbot.EditMessageTextOpts{
                    ReplyMarkup: stopButton,
                })
                if err != nil {
                    log.Printf("Error updating progress: %v", err)
                }
            }

            time.Sleep(50 * time.Millisecond)
        }
    }

    broadcastMu.Lock()
    broadcastActive = false
    broadcastMu.Unlock()

    statusMsg.Delete(b, nil)
    finalText := fmt.Sprintf(
        "✅ Broadcast Completed!\n\n"+
            "Total Users: %d\n"+
            "✅ Success: %d\n"+
            "❌ Failed: %d",
        total, success, failed,
    )
    _, err = ctx.EffectiveMessage.Reply(b, finalText, nil)

    return err
}

func AllBroadcast(b *gotgbot.Bot, ctx *ext.Context) error {
    if !isAdmin(ctx.EffectiveUser.Id) {
        return nil
    }

    if !database.IsMongoAvailable() {
        _, err := ctx.EffectiveMessage.Reply(b, "❌ MongoDB is not configured.", nil)
        return err
    }

    args := strings.SplitN(ctx.EffectiveMessage.Text, " ", 2)
    if len(args) < 2 {
        _, err := ctx.EffectiveMessage.Reply(b, "❌ Please provide a message to broadcast.\nUsage: /allbroadcast message", nil)
        return err
    }

    messageText := args[1]

    statusMsg, err := ctx.EffectiveMessage.Reply(b, "📢 Broadcasting to all bot users...", nil)
    if err != nil {
        return err
    }

    cloneBots, err := database.GetAllCloneBots(context.Background())
    if err != nil {
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error getting bots: %v", err), nil)
        return err
    }

    if len(cloneBots) == 0 {
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, "❌ No clone bots found.", nil)
        return nil
    }

    mainUsers, err := database.GetAllUsers(context.Background())
    if err != nil {
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error getting users: %v", err), nil)
        return err
    }

    if len(mainUsers) == 0 {
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, "❌ No users found in database.", nil)
        return nil
    }

    totalSent := 0
    failedBots := 0
    totalMessages := 0

    for _, bot := range cloneBots {
        botToken := fmt.Sprintf("%d:", bot.BotID)
        botClient, err := gotgbot.NewBot(botToken, &gotgbot.BotOpts{
            DisableTokenCheck: true,
        })
        if err != nil {
            failedBots++
            continue
        }

        botSuccess := 0
        for _, user := range mainUsers {
            _, err := botClient.SendMessage(user.UserID, messageText, &gotgbot.SendMessageOpts{
                ParseMode: "HTML",
            })
            if err == nil {
                botSuccess++
                totalSent++
            }
            time.Sleep(50 * time.Millisecond)
        }
        totalMessages += botSuccess
        
        if botSuccess == 0 {
            failedBots++
        }
        
        time.Sleep(100 * time.Millisecond)
    }

    statusMsg.Delete(b, nil)
    finalText := fmt.Sprintf(
        "✅ Broadcast Completed!\n\n"+
            "🤖 Total Bots: %d\n"+
            "📨 Messages Sent: %d\n"+
            "❌ Failed Bots: %d",
        len(cloneBots), totalSent, failedBots,
    )
    _, err = ctx.EffectiveMessage.Reply(b, finalText, nil)
    return err
}

func RestartAllBots(b *gotgbot.Bot, ctx *ext.Context) error {
    if !isAdmin(ctx.EffectiveUser.Id) {
        return nil
    }

    if !database.IsMongoAvailable() {
        _, err := ctx.EffectiveMessage.Reply(b, "❌ MongoDB is not configured.", nil)
        return err
    }

    statusMsg, err := ctx.EffectiveMessage.Reply(b, "🔄 Restarting all clone bots...\n\nProgress: 0%", nil)
    if err != nil {
        return err
    }

    cloneBots, err := database.GetAllCloneBots(context.Background())
    if err != nil {
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error getting bots: %v", err), nil)
        return err
    }

    if len(cloneBots) == 0 {
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, "❌ No clone bots found.", nil)
        return nil
    }

    webhookURL := os.Getenv("WEBHOOK_URL")
    if webhookURL == "" {
        statusMsg.Delete(b, nil)
        ctx.EffectiveMessage.Reply(b, "❌ WEBHOOK_URL environment variable not set.", nil)
        return nil
    }

    webhookURL = strings.TrimSuffix(webhookURL, "/")
    total := len(cloneBots)
    success := 0
    failed := 0
    removed := 0
    botsToRemove := []int64{}

    for i, cloneBot := range cloneBots {
        botToken := fmt.Sprintf("%d:", cloneBot.BotID)
        botClient, err := gotgbot.NewBot(botToken, &gotgbot.BotOpts{
            DisableTokenCheck: true,
        })
        if err != nil {
            failed++
            botsToRemove = append(botsToRemove, cloneBot.BotID)
            
            percentage := (i + 1) * 100 / total
            progressText := fmt.Sprintf(
                "🔄 Restarting all clone bots...\n\n"+
                    "Progress: %d%% (%d/%d)\n"+
                    "✅ Success: %d\n"+
                    "❌ Failed: %d\n"+
                    "🗑️ Removed: %d",
                percentage, i+1, total, success, failed, removed,
            )
            statusMsg.EditText(b, progressText, nil)
            continue
        }

        botClient.DeleteWebhook(&gotgbot.DeleteWebhookOpts{
            DropPendingUpdates: true,
        })

        webhookEndpoint := fmt.Sprintf("%s/webhook/%s", webhookURL, botToken)
        _, err = botClient.SetWebhook(webhookEndpoint, &gotgbot.SetWebhookOpts{
            DropPendingUpdates: true,
        })
        
        if err != nil {
            failed++
            botsToRemove = append(botsToRemove, cloneBot.BotID)
        } else {
            success++
        }

        percentage := (i + 1) * 100 / total
        progressText := fmt.Sprintf(
            "🔄 Restarting all clone bots...\n\n"+
                "Progress: %d%% (%d/%d)\n"+
                "✅ Success: %d\n"+
                "❌ Failed: %d\n"+
                "🗑️ Removed: %d",
            percentage, i+1, total, success, failed, removed,
        )
        statusMsg.EditText(b, progressText, nil)
        
        time.Sleep(100 * time.Millisecond)
    }

    for _, botID := range botsToRemove {
        err := database.DeleteCloneBotByID(context.Background(), botID)
        if err == nil {
            removed++
        }
    }

    statusMsg.Delete(b, nil)
    
    var resultText string
    if removed > 0 {
        resultText = fmt.Sprintf(
            "✅ Restart Completed!\n\n"+
                "🤖 Total Bots: %d\n"+
                "✅ Success: %d\n"+
                "❌ Failed: %d\n"+
                "🗑️ Invalid Bots Removed: %d",
            total, success, failed, removed,
        )
    } else {
        resultText = fmt.Sprintf(
            "✅ Restart Completed!\n\n"+
                "🤖 Total Bots: %d\n"+
                "✅ Success: %d\n"+
                "❌ Failed: %d",
            total, success, failed,
        )
    }
    
    _, err = ctx.EffectiveMessage.Reply(b, resultText, nil)
    return err
}

func HandleStopBroadcast(b *gotgbot.Bot, ctx *ext.Context) error {
    query := ctx.Update.CallbackQuery

    if !isAdmin(query.From.Id) {
        _, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
            Text: "❌ You are not authorized to stop the broadcast.",
        })
        return err
    }

    broadcastMu.Lock()
    if broadcastActive {
        broadcastStop <- true
    }
    broadcastMu.Unlock()

    _, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
        Text: "🛑 Broadcast stopping...",
    })

    return err
}

func importsEval(code string) (string, string) {
    code = strings.TrimSpace(code)

    blockRe := regexp.MustCompile(`(?s)import\s*\((.*?)\)`)
    if blockRe.MatchString(code) {
        block := blockRe.FindString(code)
        code = blockRe.ReplaceAllString(code, "")
        return strings.TrimSpace(block), strings.TrimSpace(code)
    }

    singleRe := regexp.MustCompile(`(?i)import\s*\(?\s*"([^"]+)"\s*\)?`)
    if singleRe.MatchString(code) {
        importPath := singleRe.FindStringSubmatch(code)[1]
        block := "import (\n\t\"" + importPath + "\"\n)"
        code = singleRe.ReplaceAllString(code, "")
        return block, strings.TrimSpace(code)
    }

    return "", strings.TrimSpace(code)
}

func EvalCmd(b *gotgbot.Bot, ctx *ext.Context) error {
    if !isAdmin(ctx.EffectiveUser.Id) {
        return nil
    }

    msg := ctx.EffectiveMessage

    parts := strings.SplitN(msg.Text, " ", 2)
    if len(parts) != 2 {
        _, err := msg.Reply(b, "❌ Please provide code to evaluate.\nUsage: /eval code", &gotgbot.SendMessageOpts{
            ParseMode: "HTML",
        })
        return err
    }

    codef := strings.TrimSpace(parts[1])

    statusMsg, _ := msg.Reply(b, "🔄 Running code...", nil)

    msgJSON, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    impts, code := importsEval(codef)

    tmpFileName := fmt.Sprintf("eval_%d.go", time.Now().UnixNano())

    botToken := os.Getenv("TOKEN")

    fileContent := fmt.Sprintf(`package main

import (
    "encoding/json"
    "fmt"

    "github.com/PaulSonOfLars/gotgbot/v2"
)

%s

func main() {
    b, err := gotgbot.NewBot("%s", nil)
    if err != nil {
        fmt.Println("Bot Error:", err)
        return
    }

    rawJSON := `+"`"+`%s`+"`"+`

    m := new(gotgbot.Message)
    if err := json.Unmarshal([]byte(rawJSON), m); err != nil {
        fmt.Println("Unmarshal Error:", err)
        return
    }

    var r *gotgbot.Message
    if m.ReplyToMessage != nil {
        r = m.ReplyToMessage
    }

    {
        %s
    }

    _ = b
    _ = m
    _ = r
}
`, impts, botToken, string(msgJSON), code)

    if err := os.WriteFile(tmpFileName, []byte(fileContent), 0644); err != nil {
        return err
    }
    defer os.Remove(tmpFileName)

    cmd := exec.Command("go", "run", tmpFileName)

    var out bytes.Buffer
    var stderr bytes.Buffer

    cmd.Stdout = &out
    cmd.Stderr = &stderr

    runErr := cmd.Run()

    output := out.String()
    errOut := stderr.String()

    if runErr != nil && errOut == "" {
        errOut = runErr.Error()
    }

    res := fmt.Sprintf(
        "<b>📝 Code:</b>\n<pre language='go'>%s</pre>\n\n",
        html.EscapeString(codef),
    )

    if errOut != "" {
        res += fmt.Sprintf(
            "<b>❌ Error:</b>\n<pre language='go'>%s</pre>",
            html.EscapeString(errOut),
        )
    } else if output != "" {
        res += fmt.Sprintf(
            "<b>✅ Output:</b>\n<pre language='go'>%s</pre>",
            html.EscapeString(output),
        )
    } else {
        res += "<b>✅ Success</b>"
    }

    if len(res) > 4000 {
        res = res[:4000] + "\n\n... (truncated)"
    }

    _, _, err = statusMsg.EditText(b, res, &gotgbot.EditMessageTextOpts{
        ParseMode: "HTML",
    })

    return err
}
