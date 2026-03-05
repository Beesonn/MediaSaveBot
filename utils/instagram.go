package utils

import (
    "fmt"
    "log"
    "strings"
    "time"
    "context"
    "os"
    "strconv"

    "github.com/Beesonn/dlkitgo"
    "github.com/Beesonn/MediaSaveBot/database"
    "github.com/PaulSonOfLars/gotgbot/v2"
    "github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func HandleInstagram(b *gotgbot.Bot, ctx *ext.Context) error {
    url := ctx.EffectiveMessage.Text
    
    postID := extractInstagramID(url)
    
    statusMsg, err := ctx.EffectiveMessage.Reply(b, "Downloading.....", nil)
    if err != nil {
        return err
    }

    if postID != "" {
        cachedMedia, err := database.GetMedia(context.Background(), "instagram", postID)
        if err == nil && cachedMedia != nil && len(cachedMedia.FileIDs) > 0 {
            return sendCachedInstagram(b, ctx, cachedMedia, statusMsg)
        }
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

    channelIDStr := os.Getenv("CHANNEL_ID")
    var dbChannelID int64 = 0
    if channelIDStr != "" {
        dbChannelID, _ = strconv.ParseInt(channelIDStr, 10, 64)
    }
    
    fileIDs := make([]string, 0)

    if postID != "" && dbChannelID != 0 {
        channelMedia := make([]gotgbot.InputMedia, 0)
        
        for i, source := range stream.Source {
            if source.Type == "video" {
                channelMedia = append(channelMedia, gotgbot.InputMediaVideo{
                    Media:     gotgbot.InputFileByURL(source.URL),
                    Caption:   stream.Caption,
                    ParseMode: "HTML",
                })
            } else {
                channelMedia = append(channelMedia, gotgbot.InputMediaPhoto{
                    Media:     gotgbot.InputFileByURL(source.URL),
                    Caption:   stream.Caption,
                    ParseMode: "HTML",
                })
            }
        }

        sentMessages, err := b.SendMediaGroup(dbChannelID, channelMedia, nil)
        if err == nil {
            for _, msg := range sentMessages {
                if msg.Photo != nil && len(msg.Photo) > 0 {
                    fileIDs = append(fileIDs, msg.Photo[len(msg.Photo)-1].FileId)
                } else if msg.Video != nil {
                    fileIDs = append(fileIDs, msg.Video.FileId)
                }
            }
            
            if postID != "" && len(fileIDs) > 0 {
                dbMedia := &database.Media{
                    PostID:    postID,
                    Platform:  "instagram",
                    FileIDs:   fileIDs,
                    Caption:   stream.Caption,
                    Username:  stream.Username,
                    MediaType: "mixed",
                    CreatedAt: time.Now(),
                }
                database.SaveMedia(context.Background(), dbMedia)
            }
        }
    }

    statusMsg.Delete(b, nil)

    media := make([]gotgbot.InputMedia, 0)

    for i, source := range stream.Source {
        if source.Type == "video" {
            media = append(media, gotgbot.InputMediaVideo{
                Media:     gotgbot.InputFileByURL(source.URL),
                Caption:   stream.Caption,
                ParseMode: "HTML",
            })
        } else {
            media = append(media, gotgbot.InputMediaPhoto{
                Media:     gotgbot.InputFileByURL(source.URL),
                Caption:   stream.Caption,
                ParseMode: "HTML",
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

func sendCachedInstagram(b *gotgbot.Bot, ctx *ext.Context, cachedMedia *database.Media, statusMsg *gotgbot.Message) error {
    statusMsg.Delete(b, nil)

    media := make([]gotgbot.InputMedia, 0)

    for i, fileID := range cachedMedia.FileIDs {
        if cachedMedia.MediaType == "video" && i == 0 {
            media = append(media, gotgbot.InputMediaVideo{
                Media:     gotgbot.InputFileByID(fileID),
                Caption:   cachedMedia.Caption,
                ParseMode: "HTML",
            })
        } else {
            media = append(media, gotgbot.InputMediaPhoto{
                Media:     gotgbot.InputFileByID(fileID),
                Caption:   cachedMedia.Caption,
                ParseMode: "HTML",
            })
        }
    }

    _, err := b.SendMediaGroup(ctx.EffectiveChat.Id, media, &gotgbot.SendMediaGroupOpts{
        ReplyParameters: &gotgbot.ReplyParameters{
            MessageId: ctx.EffectiveMessage.MessageId,
        },
    })

    return err
}

func extractInstagramID(url string) string {
    parts := strings.Split(url, "/")
    for i, part := range parts {
        if part == "p" || part == "reel" || part == "tv" {
            if i+1 < len(parts) {
                return strings.Split(parts[i+1], "?")[0]
            }
        }
    }
    return ""
}
