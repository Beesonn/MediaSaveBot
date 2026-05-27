package main

import (
	"log"
	"os"
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

	dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
		Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
			log.Println("an error occurred while handling update:", err.Error())
			return ext.DispatcherActionNoop
		},
		MaxRoutines: ext.DefaultMaxRoutines,
	})

	updater := ext.NewUpdater(dispatcher, nil)

	dispatcher.AddHandler(handlers.NewCommand("start", bot.Start))

	if database.IsMongoAvailable() {
		dispatcher.AddHandler(handlers.NewCommand("stats", bot.Stats))
		dispatcher.AddHandler(handlers.NewCommand("broadcast", bot.Broadcast))
		dispatcher.AddHandler(handlers.NewCallback(callbackquery.Prefix("stop_broadcast"), bot.HandleStopBroadcast))
	}

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
    dispatcher.HandleMsg("eval", bot.EvalCmd)
	dispatcher.AddHandler(handlers.NewInlineQuery(inlinequery.All, bot.HandleInlineQuery))
	dispatcher.AddHandler(handlers.NewMessage(nil, bot.HandleMessage))

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
