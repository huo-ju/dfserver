package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/huo-ju/dfserver/pkg/data"
	"github.com/huo-ju/dfserver/pkg/rabbitmq"
	"google.golang.org/protobuf/proto"
)

var argslist = []string{
	"-h", "--help", "--tokenize", "-t", "--height", "-H", "--width", "-W",
	"--cfg_scale", "-C", "--number", "-n", "--separate-images", "-i", "--grid", "-g",
	"--sampler", "-A", "--steps", "-s", "--seed", "-S", "--prior", "-p"}

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
	d.s.Identify.Intents = discordgo.IntentsGuildMessages
	err = d.s.Open()
	if err != nil {
		return err
	}
	log.Println("discord bot is running.")
	select {
	case <-ctx.Done():
		log.Println("Stop discord bot...")
		d.s.Close()
		return nil
	}
}

func (d *DiscordService) messageCreate(s *discordgo.Session, mc *discordgo.MessageCreate) {
	m := mc.Message
	if m.Author.ID == s.State.User.ID {
		return
	}
	if strings.HasPrefix(m.Content, "!dream ") { //bot command
		args := data.ArgsParse(m.Content, argslist)
		task := data.DiscordCmdArgsToTask("ai.sd14", args)
		name := "process." + d.servicename
		data.AddDiscordInputTask(name, m.Reference(), task)

		body, err := proto.Marshal(task)
		if err != nil {
			fmt.Println(err)
			//TODO, response err message
		}
		priority := uint8(1)
		err = d.amqpQueue.PublishExchangePriority(*task.InputList[0].Name, "all", body, priority)
		if err != nil {
			fmt.Println(err)
			//TODO, response err message
		}
		d.s.ChannelMessageSend(m.ChannelID, "working..."+args.Cmd)
	}
}

func (d *DiscordService) ReplyMessage(channelid string, msg *discordgo.MessageSend) {
	d.s.ChannelMessageSendComplex(channelid, msg)
}
