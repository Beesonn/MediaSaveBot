package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/Beesonn/MediaSaveBot/bot"
	"github.com/Beesonn/MediaSaveBot/database"
	"github.com/Beesonn/MediaSaveBot/utils"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/callbackquery"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers/filters/inlinequery"
)

var globalDispatcher *ext.Dispatcher

func main() {
	token := os.Getenv("TOKEN")
	if token == "" {
		panic("TOKEN environment variable is empty")
	}

	if err := database.InitDB(); err != nil {
		log.Printf("Warning: Database initialization failed: %v", err)
	}

	b, err := gotgbot.NewBot(token, &gotgbot.BotOpts{
		RequestOpts: &gotgbot.RequestOpts{
			Timeout: time.Minute * 50,
		},
	})
	if err != nil {
		panic("failed to create new bot: " + err.Error())
	}

	utils.SetBotUsername(b.User.Username)

	globalDispatcher = ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	setupHandlers(globalDispatcher)

	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL != "" {
		log.Printf("Starting webhook server for clone bots on port 8080")
		http.HandleFunc("/", healthHandler)
		http.HandleFunc("/webhook/", webhookHandler)
		go func() {
			log.Fatal(http.ListenAndServe(":8080", nil))
		}()
	}

	log.Printf("Main bot %s running in polling mode...", b.User.Username)
	updater := ext.NewUpdater(globalDispatcher, nil)
	err = updater.StartPolling(b, &ext.PollingOpts{
		DropPendingUpdates: true,
		GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
			Timeout: 50,
			RequestOpts: &gotgbot.RequestOpts{
				Timeout: time.Minute * 50,
			},
		},
	})
	if err != nil {
		panic("failed to start polling: " + err.Error())
	}

	log.Printf("%s has been started...\n", b.User.Username)
	updater.Idle()
}

func setupHandlers(dispatcher *ext.Dispatcher) {
	dispatcher.AddHandler(handlers.NewCommand("start", bot.Start))
	dispatcher.AddHandler(handlers.NewCommand("song", bot.HandleSong))
	dispatcher.AddHandler(handlers.NewCommand("donate", bot.Donate))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("song_"), bot.HandleSongCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("pg_"), utils.HandlePlaylistCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("tr_"), utils.HandlePlaylistCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("dl_now_"), utils.HandlePlaylistCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("stop_dl_"), utils.HandlePlaylistCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("yt_inline_"), bot.HandleInlineYoutubeCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("yt_"), utils.HandleYoutubeCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("cancel"), utils.HandlePlaylistCallback))
	dispatcher.AddHandler(handlers.NewCallback(callbackquery.Equal("create_bot"), bot.HandleCreateBotCallback))
	dispatcher.AddHandler(handlers.NewInlineQuery(inlinequery.All, bot.HandleInlineQuery))

	if database.IsMongoAvailable() {
		dispatcher.AddHandler(handlers.NewCommand("stats", bot.Stats))
		dispatcher.AddHandler(handlers.NewCommand("broadcast", bot.Broadcast))
		dispatcher.AddHandler(handlers.NewCommand("allbroadcast", bot.AllBroadcast))
		dispatcher.AddHandler(handlers.NewCommand("restartallbots", bot.RestartAllBots))
		dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("stop_broadcast"), bot.HandleStopBroadcast))
	}
	dispatcher.AddHandler(handlers.NewMessage(nil, bot.HandleMessage))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}

func webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	urlPath := r.URL.Path
	_, botToken := path.Split(urlPath)

	if botToken == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, "Missing bot token")
		return
	}

	botClient, err := gotgbot.NewBot(botToken, &gotgbot.BotOpts{
		RequestOpts: &gotgbot.RequestOpts{
			Timeout: time.Minute * 50,
		},
	})
	if err != nil {
		log.Printf("Failed to create bot client for token: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	var update gotgbot.Update
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Failed to read request body: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	err = json.Unmarshal(body, &update)
	if err != nil {
		log.Printf("Failed to unmarshal update: %v", err)
		w.WriteHeader(http.StatusOK)
		return
	}

	if update.Message != nil {
		log.Printf("Webhook received message from user %d in bot %s", update.Message.From.Id, botClient.User.Username)
	}

	err = globalDispatcher.ProcessUpdate(botClient, &update, nil)
	if err != nil {
		log.Printf("Error processing update: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "OK")
}
