package utils

import (
    "fmt"

    "github.com/Beesonn/dlkitgo"
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func HandleInstagram(b *gotgbot.Bot, ctx *ext.Context) error {
    url := ctx.EffectiveMessage.Text
    
    statusMsg, err := ctx.EffectiveMessage.Reply(b, "Downloading.....", nil)
    if err != nil {
        return err
    }

    client := dlkitgo.NewClient()
    
    stream, err := client.Instagram.Stream(url)
    if err != nil {
        statusMsg.Delete(b, nil)
        _, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error processing Instagram link: %v", err), nil)
        return err
    }

    if len(stream.Source) == 0 {
        statusMsg.Delete(b, nil)
        _, err = ctx.EffectiveMessage.Reply(b, "❌ No media found in this Instagram post.", nil)
        return err
    }

    statusMsg.Delete(b, nil)

    media := make([]gotgbot.InputMedia, 0)

    for _, source := range stream.Source {
        if source.Type == "video" {
            media = append(media, gotgbot.InputMediaVideo{
                Media: gotgbot.InputFileByURL(source.URL),
            })
        } else {
            media = append(media, gotgbot.InputMediaPhoto{
                Media: gotgbot.InputFileByURL(source.URL),
            })
        }
    }

    _, err = b.SendMediaGroup(ctx.EffectiveChat.Id, media, &gotgbot.SendMediaGroupOpts{
        ReplyParameters: &gotgbot.ReplyParameters{
            MessageId: ctx.EffectiveMessage.MessageId,
        },
    })

    return err
}
