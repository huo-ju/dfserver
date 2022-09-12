package data

import (
	"encoding/json"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestDiscordToTask(t *testing.T) {
	cmd1 := "!dream half body portrait drawing of anime muscular horse, epic pose, pen and ink, intricate line drawings, by craig mullins, ruan jia, kentaro miura, greg rutkowski, loundraw and dan mumford -W 768 -n 4 -s 30"
	args := ArgsParse(cmd1, ArgsList)
	task := DiscordCmdArgsToTask("ai.sd14", args)

	jsonstr := "{\"prompt\":\"half body portrait drawing of anime muscular horse, epic pose, pen and ink, intricate line drawings, by craig mullins, ruan jia, kentaro miura, greg rutkowski, loundraw and dan mumford\",\"number\":4,\"height\":0,\"width\":768,\"num_inference_steps\":30,\"guidance_scale\":0,\"eta\":0}"
	if len(task.InputList) != 1 {
		t.Errorf("err, expect number of InputList: 1 ,result: %d ", len(task.InputList))
	}
	if string(task.InputList[0].Settings) != jsonstr {
		t.Errorf("err, expect settings string: %s ,result: %s ", jsonstr, task.InputList[0].Settings)
	}

	name := "process.discord.server1"
	reference := &discordgo.MessageReference{MessageID: "a_test_msgid", ChannelID: "a_test_chanid", GuildID: "a_test_guildid"}
	AddDiscordInputTask(name, reference, task)

	if len(task.InputList) != 2 {
		t.Errorf("err, expect number of InputList: 2 ,result: %d ", len(task.InputList))
	}
	if *task.InputList[1].Name != name {
		t.Errorf("err, expect inputTask name: %s,result: %s ", name, *task.InputList[1].Name)
	}
	refstr, _ := json.Marshal(reference)
	if string(task.InputList[1].Settings) != string(refstr) {
		t.Errorf("err, expect settings string: %s ,result: %s ", jsonstr, task.InputList[0].Settings)
	}

}
