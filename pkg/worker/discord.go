package worker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	dfpb "github.com/huo-ju/dfserver/pkg/pb"
	"github.com/huo-ju/dfserver/pkg/service"
)

type ProcessDiscordWorker struct {
	ds *service.DiscordService
}

func (f *ProcessDiscordWorker) Name() string {
	return "process.discord"
}

//lastoutput *dfpb.Output
func (f *ProcessDiscordWorker) Work(outputList []*dfpb.Output, lastinput *dfpb.Input, settingsdata []byte) (bool, error) {
	var settings map[string]interface{}
	err := json.Unmarshal(settingsdata, &settings)
	if err != nil {
		//TODO: save err log
		return true, err
	}
	messageid := settings["message_id"].(string)
	channelid := settings["channel_id"].(string)
	guildid := settings["guild_id"].(string)
	ref := &discordgo.MessageReference{MessageID: messageid, ChannelID: channelid, GuildID: guildid}
	content := ""

	lastoutput := outputList[len(outputList)-1]

	if *lastoutput.MimeType == "text/plain" {
		if lastoutput.Error != nil {
			content = fmt.Sprintf("Error: %s by %s\r", *lastoutput.Error, *lastoutput.ProducerName)
		} else {
			content = fmt.Sprintf("%s by %s\r", string(lastoutput.Data), *lastoutput.ProducerName)
		}
		msg := &discordgo.MessageSend{
			Content:   content,
			Reference: ref,
		}
		f.ds.ReplyMessage(channelid, msg)
		return true, err
	}

	//bot response images
	r := bytes.NewReader(lastoutput.Data)
	filename := ""
	for _, o := range outputList {
		content += fmt.Sprintf("!dream %s | by %s\r", string(o.Args), *o.ProducerName)
		filename += string(o.Args)
	}

	filename = strings.Replace(filename, " ", "_", -1)
	if len(filename) > 250 { //max filename length 250
		filename = filename[len(filename)-250:]
	}
	if len(filename) == 0 {
		filename = "output"
	}

	msg := &discordgo.MessageSend{
		Content:   content,
		File:      &discordgo.File{Name: filename + ".png", Reader: r},
		Reference: ref,
	}

	if *lastinput.Name == "ai.sd14" {
		msg.Components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Emoji: discordgo.ComponentEmoji{
							Name: "",
						},
						Label:    "Upscale 4X",
						CustomID: "bt_upscale",
						Style:    discordgo.SuccessButton,
					},
					discordgo.Button{
						Emoji: discordgo.ComponentEmoji{
							Name: "",
						},
						Label:    "New Variant",
						CustomID: "bt_newvar",
						Style:    discordgo.SuccessButton,
					},
				},
			},
		}
	}

	f.ds.ReplyMessage(channelid, msg)
	return true, err
}
