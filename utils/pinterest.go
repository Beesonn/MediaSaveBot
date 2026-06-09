package utils

import (
	"github.com/Beesonn/dlkitgo"
	"github.com/Beesonn/dlkitgo/pinterest/providers"
	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func GetPinterestMedia(url string) ([]providers.MediaSource, string, error) {
	client := dlkitgo.NewClient()
	stream, err := client.Pinterest.Stream(url)
	if err != nil {
		return nil, "", err
	}

	return stream.Source, stream.Title, nil
}

func HandlePinterest(b *gotgbot.Bot, ctx *ext.Context) error {
	text := ctx.EffectiveMessage.Text
	url := ExtractFirstURL(text)

	if url == "" {
		ctx.EffectiveMessage.Reply(b, "❌ No valid Pinterest link found in the message.", nil)
		return nil
	}

	statusMsg, err := ctx.EffectiveMessage.Reply(b, "Downloading.....", nil)
	if err != nil {
		return err
	}

	sources, title, err := GetPinterestMedia(url)
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

	media := make([]gotgbot.InputMedia, 0)

	for _, source := range sources {
		if source.Type == "video" {
			media = append(media, gotgbot.InputMediaVideo{
				Media:   gotgbot.InputFileByURL(source.URL),
				Caption: title,
			})
		} else {
			media = append(media, gotgbot.InputMediaPhoto{
				Media:   gotgbot.InputFileByURL(source.URL),
				Caption: title,
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
