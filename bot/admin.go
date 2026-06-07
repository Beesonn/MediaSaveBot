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
		_, err := ctx.EffectiveMessage.Reply(b, "❌ MongoDB is not configured.", nil)
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
		_, err := ctx.EffectiveMessage.Reply(b, "❌ MongoDB is not configured.", nil)
		return err
	}

	broadcastMu.Lock()
	if broadcastActive {
		broadcastMu.Unlock()
		_, err := ctx.EffectiveMessage.Reply(b, "⚠️ A broadcast is already in progress.", nil)
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
		ctx.EffectiveMessage.Reply(b, "❌ No users found.", nil)
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
					"Total: %d\n"+
					"✅ Success: %d\n"+
					"❌ Failed: %d",
				total, success, failed,
			)
			ctx.EffectiveMessage.Reply(b, finalText, nil)
			return nil

		default:
			_, err := b.CopyMessage(user.UserID, ctx.EffectiveChat.Id, ctx.EffectiveMessage.ReplyToMessage.MessageId, nil)

			if err != nil {
				failed++
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
		_, err := ctx.EffectiveMessage.Reply(b, "❌ Usage: /allbroadcast message", nil)
		return err
	}

	messageText := args[1]

	statusMsg, err := ctx.EffectiveMessage.Reply(b, "📢 Broadcasting to all clone bot users...\n\nProgress: 0%", nil)
	if err != nil {
		return err
	}

	cloneBots, err := database.GetAllCloneBots(context.Background())
	if err != nil {
		statusMsg.Delete(b, nil)
		ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error: %v", err), nil)
		return err
	}

	if len(cloneBots) == 0 {
		statusMsg.Delete(b, nil)
		ctx.EffectiveMessage.Reply(b, "❌ No clone bots found.", nil)
		return nil
	}

	totalBots := len(cloneBots)
	processedBots := 0
	totalSent := 0
	failedBots := 0

	for _, bot := range cloneBots {
		processedBots++

		if bot.BotToken == "" {
			log.Printf("❌ Bot @%s has empty token", bot.Username)
			failedBots++

			percentage := processedBots * 100 / totalBots
			progressText := fmt.Sprintf(
				"📢 Broadcasting to all clone bot users...\n\n"+
					"Progress: %d%% (%d/%d bots)\n"+
					"📨 Messages Sent: %d\n"+
					"❌ Failed Bots: %d",
				percentage, processedBots, totalBots, totalSent, failedBots,
			)
			statusMsg.EditText(b, progressText, nil)
			continue
		}

		botClient, err := gotgbot.NewBot(bot.BotToken, &gotgbot.BotOpts{
			DisableTokenCheck: true,
		})
		if err != nil {
			log.Printf("❌ Failed to create bot client for @%s: %v", bot.Username, err)
			failedBots++

			percentage := processedBots * 100 / totalBots
			progressText := fmt.Sprintf(
				"📢 Broadcasting to all clone bot users...\n\n"+
					"Progress: %d%% (%d/%d bots)\n"+
					"📨 Messages Sent: %d\n"+
					"❌ Failed Bots: %d",
				percentage, processedBots, totalBots, totalSent, failedBots,
			)
			statusMsg.EditText(b, progressText, nil)
			continue
		}

		users, err := database.GetCloneBotUsers(bot.BotID)
		if err != nil {
			log.Printf("❌ Failed to get users for bot @%s: %v", bot.Username, err)
			failedBots++

			percentage := processedBots * 100 / totalBots
			progressText := fmt.Sprintf(
				"📢 Broadcasting to all clone bot users...\n\n"+
					"Progress: %d%% (%d/%d bots)\n"+
					"📨 Messages Sent: %d\n"+
					"❌ Failed Bots: %d",
				percentage, processedBots, totalBots, totalSent, failedBots,
			)
			statusMsg.EditText(b, progressText, nil)
			continue
		}

		if len(users) == 0 {
			percentage := processedBots * 100 / totalBots
			progressText := fmt.Sprintf(
				"📢 Broadcasting to all clone bot users...\n\n"+
					"Progress: %d%% (%d/%d bots)\n"+
					"📨 Messages Sent: %d\n"+
					"❌ Failed Bots: %d",
				percentage, processedBots, totalBots, totalSent, failedBots,
			)
			statusMsg.EditText(b, progressText, nil)
			continue
		}

		botSuccess := 0
		for idx, user := range users {
			_, err := botClient.SendMessage(user.UserID, messageText, &gotgbot.SendMessageOpts{
				ParseMode: "HTML",
			})
			if err != nil {
				log.Printf("❌ Bot @%s failed to send message to user %d: %v", bot.Username, user.UserID, err)
				time.Sleep(100 * time.Millisecond)
			} else {
				botSuccess++
				totalSent++

				if idx%3 == 0 || idx == len(users)-1 {
					percentage := processedBots * 100 / totalBots
					progressText := fmt.Sprintf(
						"📢 Broadcasting to all clone bot users...\n\n"+
							"Progress: %d%% (%d/%d bots)\n"+
							"📨 Messages Sent: %d\n"+
							"❌ Failed Bots: %d\n"+
							"📤 Bot @%s: %d/%d users",
						percentage, processedBots, totalBots, totalSent, failedBots, bot.Username, botSuccess, len(users),
					)
					statusMsg.EditText(b, progressText, nil)
				}
			}

			time.Sleep(150 * time.Millisecond)
		}

		if botSuccess == 0 {
			failedBots++
		}

		percentage := processedBots * 100 / totalBots
		progressText := fmt.Sprintf(
			"📢 Broadcasting to all clone bot users...\n\n"+
				"Progress: %d%% (%d/%d bots)\n"+
				"📨 Messages Sent: %d\n"+
				"❌ Failed Bots: %d",
			percentage, processedBots, totalBots, totalSent, failedBots,
		)
		statusMsg.EditText(b, progressText, nil)

		time.Sleep(500 * time.Millisecond)
	}

	statusMsg.Delete(b, nil)

	finalText := fmt.Sprintf(
		"✅ Broadcast Completed!\n\n"+
			"🤖 Total Bots: %d\n"+
			"📨 Messages Sent: %d\n"+
			"❌ Failed Bots: %d",
		totalBots, totalSent, failedBots,
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
		ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error: %v", err), nil)
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
		ctx.EffectiveMessage.Reply(b, "❌ WEBHOOK_URL not set.", nil)
		return nil
	}

	webhookURL = strings.TrimSuffix(webhookURL, "/")
	total := len(cloneBots)
	success := 0
	failed := 0
	removed := 0
	botsToRemove := []int64{}

	for i, cloneBot := range cloneBots {
		if cloneBot.BotToken == "" {
			failed++
			botsToRemove = append(botsToRemove, cloneBot.BotID)

			percentage := (i + 1) * 100 / total
			progressText := fmt.Sprintf(
				"🔄 Restarting...\n\n"+
					"Progress: %d%% (%d/%d)\n"+
					"✅ Success: %d\n"+
					"❌ Failed: %d\n"+
					"🗑️ Removed: %d",
				percentage, i+1, total, success, failed, removed,
			)
			statusMsg.EditText(b, progressText, nil)
			continue
		}

		botClient, err := gotgbot.NewBot(cloneBot.BotToken, &gotgbot.BotOpts{
			DisableTokenCheck: true,
		})
		if err != nil {
			failed++
			botsToRemove = append(botsToRemove, cloneBot.BotID)

			percentage := (i + 1) * 100 / total
			progressText := fmt.Sprintf(
				"🔄 Restarting...\n\n"+
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

		webhookEndpoint := fmt.Sprintf("%s/webhook/%s", webhookURL, cloneBot.BotToken)
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
			"🔄 Restarting...\n\n"+
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
			Text: "❌ Not authorized.",
		})
		return err
	}

	broadcastMu.Lock()
	if broadcastActive {
		broadcastStop <- true
	}
	broadcastMu.Unlock()

	_, err := query.Answer(b, &gotgbot.AnswerCallbackQueryOpts{
		Text: "🛑 Stopping broadcast...",
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
		_, err := msg.Reply(b, "❌ Usage: /eval code", &gotgbot.SendMessageOpts{
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
