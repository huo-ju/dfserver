package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/huo-ju/dfserver/pkg/data"
	dfpb "github.com/huo-ju/dfserver/pkg/pb"
	"github.com/huo-ju/dfserver/pkg/rabbitmq"
	"google.golang.org/protobuf/proto"
)

type DiscordService struct {
	servicename string
	token       string
	prefix      string
	amqpQueue   *rabbitmq.AmqpQueue
	s           *discordgo.Session
}

func NewDiscordService(servicename string, token string, prefix string, amqpQueue *rabbitmq.AmqpQueue) *DiscordService {
	d := &DiscordService{servicename: servicename, token: token, prefix: prefix, amqpQueue: amqpQueue}
	return d
}

func (d *DiscordService) Start(ctx context.Context) error {
	var err error
	d.s, err = discordgo.New("Bot " + d.token)
	if err != nil {
		return err
	}
	d.s.AddHandler(d.messageCreate)
	d.s.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.MessageComponentData().CustomID == "bt_upscale" {
			if len(i.Message.Attachments) == 1 { //TODO: >1 multi images upscale ?
				buff, err := data.DownloadFile(i.Message.Attachments[0].ProxyURL)
				if err == nil {
					buffbytes := buff.Bytes()
					inputtask := data.CreateImageUpscaleInputTask(&buffbytes)
					inputList := []*dfpb.Input{inputtask}
					task := data.CreateTask(inputList, nil)
					name := "process." + d.servicename
					data.AddDiscordInputTask(name, i.Message.MessageReference, task)
					body, err := proto.Marshal(task)
					if err != nil {
						fmt.Println(err)
						//TODO, response err message
					}
					priority := uint8(1)
					err = d.amqpQueue.PublishExchangePriority(task.InputList[0].Name, "all", body, priority)
					if err != nil {
						fmt.Println(err)
						//TODO, response err message
					}
					d.s.ChannelMessageSend(i.ChannelID, "upscale working...")
				} else {
					//TODO, response err message
					fmt.Println(err)
				}

			}
			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "upscaling...",
				},
			})
			if err != nil {
				//TODO: error handle
			}
		} else if i.MessageComponentData().CustomID == "bt_newvar" {
			//remove by and sig
			var re = regexp.MustCompile("\\| by.+$")
			content := re.ReplaceAllString(i.Message.Content, "")
			args := data.ArgsParse(content, data.ArgsList)
			//remove seed
			//TODO: re attach the init_image
			task, publishkey := data.CreateSDTaskWithCmdArgs(args, nil, "", true)
			name := "process." + d.servicename
			data.AddDiscordInputTask(name, i.Message.MessageReference, task)

			body, err := proto.Marshal(task)
			if err != nil {
				fmt.Println(err)
				//TODO, response err message
			}
			priority := uint8(1)
			err = d.amqpQueue.PublishExchangePriority(task.InputList[0].Name, publishkey, body, priority)
			if err != nil {
				fmt.Println(err)
				//TODO, response err message
			}

			err = s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "new variant generating",
				},
			})
		}
	})
	d.s.Identify.Intents = discordgo.IntentsGuildMessages
	err = d.s.Open()
	if err != nil {
		return err
	}
	log.Printf("discord bot %s is running.", d.servicename)
	select {
	case <-ctx.Done():
		log.Println("Stop discord bot...")
		d.s.Close()
		return nil
	}
}

func GetImageFromAttachment(att *discordgo.MessageAttachment) (*bytes.Buffer, string, error) {
	if att.ContentType == "image/jpeg" || att.ContentType == "image/png" {
		buff, err := data.DownloadFile(att.ProxyURL)
		return buff, att.ProxyURL, err

	} else {
		return nil, "", errors.New("Error: only support png and jpg image")
	}
}

func (d *DiscordService) messageCreate(s *discordgo.Session, mc *discordgo.MessageCreate) {
	m := mc.Message
	if m.Author.ID == s.State.User.ID {
		return
	}
	if strings.HasPrefix(m.Content, "!dream ") { //bot command
		args := data.ArgsParse(m.Content, data.ArgsList)

		var buff *bytes.Buffer
		var err error
		var url string
		if len(m.Attachments) > 0 { //may an image uploaded
			att := m.Attachments[0]
			buff, url, err = GetImageFromAttachment(att)
			if err != nil {
				msg := &discordgo.MessageSend{
					Content:   err.Error(),
					Reference: m.Reference(),
				}
				d.ReplyMessage(m.ChannelID, msg)
			}
		}

		name := "process." + d.servicename
		task, publishkey := data.CreateSDTaskWithCmdArgs(args, buff, url, false)
		data.AddDiscordInputTask(name, m.Reference(), task)
		body, err := proto.Marshal(task)
		if err != nil {
			fmt.Println(err)
			//TODO, response err message
		}
		priority := uint8(1)
		err = d.amqpQueue.PublishExchangePriority(task.InputList[0].Name, publishkey, body, priority)
		if err != nil {
			fmt.Println(err)
			//TODO, response err message
		}
		d.s.ChannelMessageSend(m.ChannelID, "working..."+args.Cmd)
	} else if strings.HasPrefix(m.Content, "!guess") && len(m.Attachments) > 0 { //bot command
		att := m.Attachments[0]
		buff, _, err := GetImageFromAttachment(att)
		if err != nil {
			msg := &discordgo.MessageSend{
				Content:   err.Error(),
				Reference: m.Reference(),
			}
			d.ReplyMessage(m.ChannelID, msg)
		} else {
			buffbytes := buff.Bytes()
			inputtask := data.CreateInterrogatorInputTask(&buffbytes)
			inputList := []*dfpb.Input{inputtask}
			task := data.CreateTask(inputList, nil)
			name := "process." + d.servicename
			data.AddDiscordInputTask(name, m.Reference(), task)
			body, err := proto.Marshal(task)
			if err != nil {
				fmt.Println(err)
				//TODO, response err message
			}
			priority := uint8(1)
			err = d.amqpQueue.PublishExchangePriority(task.InputList[0].Name, "all", body, priority)
			if err != nil {
				fmt.Println(err)
				//TODO, response err message
			}
			d.s.ChannelMessageSend(m.ChannelID, "guessing...")
		}
	} else if strings.HasPrefix(m.Content, "!build ") { //bot command
		task := data.CreateGptNeoTask(m.Content)
		name := "process." + d.servicename
		data.AddDiscordInputTask(name, m.Reference(), task)

		body, err := proto.Marshal(task)
		if err != nil {
			fmt.Println(err)
			//TODO, response err message
		}
		priority := uint8(1)
		err = d.amqpQueue.PublishExchangePriority(task.InputList[0].Name, "all", body, priority)
		if err != nil {
			fmt.Println(err)
			//TODO, response err message
		}
		d.s.ChannelMessageSend(m.ChannelID, "working..."+m.Content)
	}
}

func (d *DiscordService) ReplyMessage(channelid string, msg *discordgo.MessageSend) {
	d.s.ChannelMessageSendComplex(channelid, msg)
}
