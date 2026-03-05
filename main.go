package main

import (
    "log"
    "os"
    "time"

    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
    "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
    "github.com/Beesonn/MediaSaveBot/bot"
)

func main() {
    token := os.Getenv("BOT_TOKEN")
    if token == "" {
        panic("TOKEN environment variable is empty")
    }

    b, err := gotgbot.NewBot(token, nil)
    if err != nil {
        panic("failed to create new bot: " + err.Error())
    }

    dispatcher := ext.NewDispatcher(&ext.DispatcherOpts{
        Error: func(b *gotgbot.Bot, ctx *ext.Context, err error) ext.DispatcherAction {
            log.Println("an error occurred while handling update:", err.Error())
            return ext.DispatcherActionNoop
        },
        MaxRoutines: ext.DefaultMaxRoutines,
    })
    
    updater := ext.NewUpdater(dispatcher, nil)
    dispatcher.AddHandler(handlers.NewCommand("start", bot.Start))
    dispatcher.AddHandler(handlers.NewCommand("broadcast", bot.Broadcast))

    err = updater.StartPolling(b, &ext.PollingOpts{
        DropPendingUpdates: true,
        GetUpdatesOpts: &gotgbot.GetUpdatesOpts{
            Timeout: 9,
            RequestOpts: &gotgbot.RequestOpts{
                Timeout: time.Second * 10,
            },
        },
    })
    if err != nil {
        panic("failed to start polling: " + err.Error())
    }
    log.Printf("%s has been started...\n", b.User.Username)

    updater.Idle()
}
