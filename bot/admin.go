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
	"strconv"
	"strings"
	"sync"
	"time"
	"regexp"

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

func Stats(b *gotgbot.Bot, ctx *ext.Context) error {
	if !isAdmin(ctx.EffectiveUser.Id) {
		return nil
	}

	if !database.IsMongoAvailable() {
		_, err := ctx.EffectiveMessage.Reply(b, "❌ MongoDB is not configured. Please set MONGODB_URI environment variable to use this command.", nil)
		return err
	}

	count, err := database.GetUserCount(context.Background())
	if err != nil {
		_, err := ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error getting stats: %v", err), nil)
		return err
	}

	text := fmt.Sprintf(
		"📊 <b>Bot Statistics</b>\n\n"+
			"👥 <b>Total Users:</b> %d\n",
		count,
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
		_, _, err := statusMsg.EditText(b, fmt.Sprintf("❌ Error getting users: %v", err), nil)
		return err
	}

	if len(users) == 0 {
		broadcastMu.Lock()
		broadcastActive = false
		broadcastMu.Unlock()
		_, _, err := statusMsg.EditText(b, "❌ No users found in database.", nil)
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
		return nil
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

	%s
	
	_ = b
	_ = m
	_ = r
}
`, impts, botToken, string(msgJSON), code)

	if err := os.WriteFile(tmpFileName, []byte(fileContent), 0644); err != nil {
		return nil
	}
	defer os.Remove(tmpFileName)

	cmd := exec.Command("go", "run", tmpFileName)
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	runErr := cmd.Run()

	output := out.String()
	errOut := stderr.String()

	res := fmt.Sprintf("<b>📝 Code:</b>\n<pre language='go'>%s</pre>\n\n", html.EscapeString(codef))

	if runErr != nil || errOut != "" {
		res += fmt.Sprintf("<b>❌ Error:</b>\n<pre language='go'>%s</pre>", html.EscapeString(errOut))
	} else if output != "" {
		res += fmt.Sprintf("<b>✅ Output:</b>\n<pre language='go'>%s</pre>", html.EscapeString(output))
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
