package worker

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/bwmarrin/discordgo"
	"github.com/huo-ju/dfserver/pkg/data"
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

	if lastoutput.MimeType == "text/plain" {
		if lastoutput.Error != "" {
			content = fmt.Sprintf("Error: %s by %s\r", lastoutput.Error, lastoutput.ProducerName)
		} else {
			if lastinput.Name == "ai.gptneo" {
				content = fmt.Sprintf("!dream %s | by %s\r", string(lastoutput.Data), lastoutput.ProducerName)
			} else {
				content = fmt.Sprintf("%s | by %s\r", string(lastoutput.Data), lastoutput.ProducerName)
			}
		}
		msg := &discordgo.MessageSend{
			Content:   content,
			Reference: ref,
		}
		AttachButton(lastinput.Name, msg)
		f.ds.ReplyMessage(channelid, msg)
		return true, err
	}

	//bot response images
	r := bytes.NewReader(lastoutput.Data)
	filename := ""
	imgdesc := ""
	producer := ""
	for _, o := range outputList {
		content += fmt.Sprintf("!dream %s | by %s\r", string(o.Args), o.ProducerName)
		filename += string(o.Args)
		imgdesc += string(o.Args)
		if len(producer) > 0 {
			producer += " "
		}
		producer += o.ProducerName
	}
	//add tExt to png
	nr, err := data.AddImageMetaData(r, imgdesc, producer)
	if err != nil {
		nr = bytes.NewReader(lastoutput.Data)
	}

	if len(filename) > 200 { //max filename length 200
		filename = filename[len(filename)-200:]
	}
	if len(filename) == 0 {
		filename = "output"
	} else {
		filename = StripFilename(filename)
	}

	msg := &discordgo.MessageSend{
		Content:   content,
		File:      &discordgo.File{Name: filename + ".png", Reader: nr},
		Reference: ref,
	}
	AttachButton(lastinput.Name, msg)

	f.ds.ReplyMessage(channelid, msg)
	return true, err
}

func AttachButton(taskname string, msg *discordgo.MessageSend) {
	if taskname == "ai.sd14" {
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
	} else if taskname == "ai.gptneo" {
		msg.Components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Emoji: discordgo.ComponentEmoji{
							Name: "",
						},
						Label:    "Dream",
						CustomID: "bt_newvar",
						Style:    discordgo.SuccessButton,
					},
				},
			},
		}
	}
}
