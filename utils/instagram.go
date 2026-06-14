package utils

import (
	"github.com/Beesonn/dlkitgo"
	"github.com/Beesonn/dlkitgo/instagram/providers"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func GetInstagramMedia(url string) ([]providers.MediaSource, string, string, error) {
	client := dlkitgo.NewClient()
	stream, err := client.Instagram.Stream(url)
	if err != nil {
		return nil, "", "", err
	}

	thumbnail := ""
	if len(stream.Source) > 0 {
		thumbnail = stream.Source[0].Thumbnail
	}
	return stream.Source, stream.Caption, thumbnail, nil
}

func HandleInstagram(b *gotgbot.Bot, ctx *ext.Context) error {
	text := ctx.EffectiveMessage.Text
	url := ExtractFirstURL(text)

	if url == "" {
		ctx.EffectiveMessage.Reply(b, "❌ No valid Instagram link found in the message.", nil)
		return nil
	}

	statusMsg, err := ctx.EffectiveMessage.Reply(b, "Downloading.....", nil)
	if err != nil {
		return err
	}

	sources, caption, _, err := GetInstagramMedia(url)
	if err != nil {
		statusMsg.Delete(b, nil)
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	if len(sources) == 0 {
		statusMsg.Delete(b, nil)
		ctx.EffectiveMessage.Reply(b, "❌ Something went wrong. Please try again or contact our support group @XBOTSUPPORTS", nil)
		return nil
	}

	statusMsg.Delete(b, nil)

	if len(sources) == 1 {
		source := sources[0]
		if source.Type == "video" {
			_, err = b.SendVideo(ctx.EffectiveChat.Id, gotgbot.InputFileByURL(source.URL), &gotgbot.SendVideoOpts{
				Caption: caption,
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId: ctx.EffectiveMessage.MessageId,
				},
			})
		} else {
			_, err = b.SendPhoto(ctx.EffectiveChat.Id, gotgbot.InputFileByURL(source.URL), &gotgbot.SendPhotoOpts{
				Caption: caption,
				ReplyParameters: &gotgbot.ReplyParameters{
					MessageId: ctx.EffectiveMessage.MessageId,
				},
			})
		}
		return err
	}

	media := make([]gotgbot.InputMedia, 0)
	for _, source := range sources {
		if source.Type == "video" {
			media = append(media, gotgbot.InputMediaVideo{
				Media:   gotgbot.InputFileByURL(source.URL),
				Caption: caption,
			})
		} else {
			media = append(media, gotgbot.InputMediaPhoto{
				Media:   gotgbot.InputFileByURL(source.URL),
				Caption: caption,
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
