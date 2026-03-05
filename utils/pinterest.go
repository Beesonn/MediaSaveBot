package utils

import (
    "fmt"
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

func HandlePinterest(b *gotgbot.Bot, ctx *ext.Context) error {
    url := ctx.EffectiveMessage.Text
    
    pinID := extractPinterestID(url)
    
    statusMsg, err := ctx.EffectiveMessage.Reply(b, "Downloading.....", nil)
    if err != nil {
        return err
    }

    if pinID != "" {
        cachedMedia, err := database.GetMedia(context.Background(), "pinterest", pinID)
        if err == nil && cachedMedia != nil && len(cachedMedia.FileIDs) > 0 {
            return sendCachedPinterest(b, ctx, cachedMedia, statusMsg)
        }
    }

    client := dlkitgo.NewClient()
    
    stream, err := client.Pinterest.Stream(url)
    if err != nil {
        statusMsg.Delete(b, nil)
        _, err = ctx.EffectiveMessage.Reply(b, fmt.Sprintf("❌ Error processing Pinterest link: %v", err), nil)
        return err
    }

    if len(stream.Source) == 0 {
        statusMsg.Delete(b, nil)
        _, err = ctx.EffectiveMessage.Reply(b, "❌ No media found in this Pinterest pin.", nil)
        return err
    }

    channelIDStr := os.Getenv("CHANNEL_ID")
    var dbChannelID int64 = 0
    if channelIDStr != "" {
        dbChannelID, _ = strconv.ParseInt(channelIDStr, 10, 64)
    }
    
    fileIDs := make([]string, 0)

    if pinID != "" && dbChannelID != 0 {
        channelMedia := make([]gotgbot.InputMedia, 0)
        
        for _, source := range stream.Source {
            if source.Type == "video" {
                channelMedia = append(channelMedia, gotgbot.InputMediaVideo{
                    Media:     gotgbot.InputFileByURL(source.URL),
                    Caption:   stream.Title,
                    ParseMode: "HTML",
                })
            } else {
                channelMedia = append(channelMedia, gotgbot.InputMediaPhoto{
                    Media:     gotgbot.InputFileByURL(source.URL),
                    Caption:   stream.Title,
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
            
            if pinID != "" && len(fileIDs) > 0 {
                mediaType := "photo"
                if stream.Source[0].Type == "video" {
                    mediaType = "video"
                }
                
                dbMedia := &database.Media{
                    PostID:    pinID,
                    Platform:  "pinterest",
                    FileIDs:   fileIDs,
                    Caption:   stream.Title,
                    Username:  "",
                    MediaType: mediaType,
                    CreatedAt: time.Now(),
                }
                database.SaveMedia(context.Background(), dbMedia)
            }
        }
    }

    statusMsg.Delete(b, nil)

    media := make([]gotgbot.InputMedia, 0)
    
    for _, source := range stream.Source {
        if source.Type == "video" {
            media = append(media, gotgbot.InputMediaVideo{
                Media:     gotgbot.InputFileByURL(source.URL),
                Caption:   stream.Title,
                ParseMode: "HTML",
            })
        } else {
            media = append(media, gotgbot.InputMediaPhoto{
                Media:     gotgbot.InputFileByURL(source.URL),
                Caption:   stream.Title,
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

func sendCachedPinterest(b *gotgbot.Bot, ctx *ext.Context, cachedMedia *database.Media, statusMsg *gotgbot.Message) error {
    statusMsg.Delete(b, nil)

    media := make([]gotgbot.InputMedia, 0)
    
    for _, fileID := range cachedMedia.FileIDs {
        if cachedMedia.MediaType == "video" {
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

func extractPinterestID(url string) string {
    if strings.Contains(url, "pin.it/") {
        parts := strings.Split(url, "pin.it/")
        if len(parts) > 1 {
            return strings.Split(parts[1], "?")[0]
        }
    }
    
    parts := strings.Split(url, "/")
    for i, part := range parts {
        if part == "pin" {
            if i+1 < len(parts) {
                return strings.Split(parts[i+1], "?")[0]
            }
        }
    }
    return ""
}
