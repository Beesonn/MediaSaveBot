package bot

import (
    "fmt"
    "context"
    "log"
    "os"
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
    // Load admin IDs from environment variable
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

func Broadcast(b *gotgbot.Bot, ctx *ext.Context) error {
    if !isAdmin(ctx.EffectiveUser.Id) {
        _, err := ctx.EffectiveMessage.Reply(b, "❌ You are not authorized to use this command.", nil)
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
        _, _, err := statusMsg.EditText(b, fmt.Sprintf("❌ Error getting users: %v", err), nil)
        return err
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
            
            finalText := fmt.Sprintf(
                "⏸️ Broadcast Stopped\n\n"+
                "Total Users: %d\n"+
                "✅ Success: %d\n"+
                "❌ Failed: %d\n"+
                "Stopped at: %d/%d",
                total, success, failed, i, total,
            )
            _, _, err := statusMsg.EditText(b, finalText, nil)
            return err
            
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
                
                _, _,  err := statusMsg.EditText(b, progressText, &gotgbot.EditMessageTextOpts{
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

    finalText := fmt.Sprintf(
        "✅ Broadcast Completed!\n\n"+
        "Total Users: %d\n"+
        "✅ Success: %d\n"+
        "❌ Failed: %d",
        total, success, failed,
    )
    _, _, err = statusMsg.EditText(b, finalText, nil)
    
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
