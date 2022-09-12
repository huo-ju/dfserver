package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/google/uuid"
	dfpb "github.com/huo-ju/dfserver/pkg/pb"
)

var ArgsList []string = []string{
	"-h", "--help", "--tokenize", "-t", "--height", "-H", "--width", "-W",
	"--cfg_scale", "-C", "--number", "-n", "--separate-images", "-i", "--grid", "-g",
	"--sampler", "-A", "--steps", "-s", "--seed", "-S", "--prior", "-p"}

func DiscordCmdArgsToTask(ainame string, args *CommandArgs) *dfpb.Task {
	settings := &DiffSettings{}
	settings.Prompt = strings.Replace(args.Cmd, "!dream ", "", 1)
	for _, a := range args.Args {
		item := strings.Split(a, " ")
		switch item[0] {
		case "--height", "-H":
			v, err := strconv.ParseUint(item[1], 10, 32)
			if err == nil {
				settings.Height = uint(v)
			}
		case "--width", "-W":
			v, err := strconv.ParseUint(item[1], 10, 32)
			if err == nil {
				settings.Width = uint(v)
			}
		case "--number", "-n":
			v, err := strconv.ParseUint(item[1], 10, 32)
			if err == nil {
				settings.Number = uint(v)
			}
		case "--steps", "-s":
			v, err := strconv.ParseUint(item[1], 10, 32)
			if err == nil {
				settings.NumInferenceSteps = uint(v)
			}
		case "--seed", "-S":
			v, err := strconv.ParseUint(item[1], 10, 64)
			if err == nil {
				settings.Seed = uint64(v)
			}
		case "--cfg_scale", "-C":
			v, err := strconv.ParseFloat(item[1], 32)
			if err == nil {
				settings.Guidance_scale = float32(v)
			}
		}
	}

	inputId := uuid.New().String()
	input := &dfpb.Input{InputId: &inputId}
	settingstr, _ := json.Marshal(settings)
	input.Name = &ainame
	input.Settings = settingstr
	inputList := []*dfpb.Input{}

	inputList = append(inputList, input)
	outputList := []*dfpb.Output{}
	taskId := uuid.New().String()
	task := &dfpb.Task{TaskId: &taskId, OutputList: outputList, InputList: inputList}
	return task
}

func AddDiscordInputTask(name string, reference *discordgo.MessageReference, task *dfpb.Task) {
	inputId := uuid.New().String()
	input := &dfpb.Input{InputId: &inputId}
	settingstr, _ := json.Marshal(reference)
	input.Settings = settingstr
	input.Name = &name
	task.InputList = append(task.InputList, input)
}

func JsonToTask(body *[]byte) (*dfpb.Task, error) {
	inputList := []*dfpb.Input{}
	outputList := []*dfpb.Output{}
	task := &dfpb.Task{OutputList: outputList}
	var tmp []interface{}
	err := json.Unmarshal(*body, &tmp)
	if err == nil {
		for _, v := range tmp {
			mv, ok := v.(map[string]interface{})
			if ok == true {
				inputId := uuid.New().String()
				input := &dfpb.Input{InputId: &inputId}
				settingstr, ok := mv["settings"].(string)
				if ok == true {
					input.Settings = []byte(settingstr)
				}

				namestr, ok := mv["name"].(string)
				if ok == true {
					input.Name = &namestr
				}

				inputList = append(inputList, input)
			} else {
				return nil, errors.New("can't parse input data")
			}
		}
		task.InputList = inputList
		taskId := uuid.New().String()
		task.TaskId = &taskId
		return task, nil
	}
	return nil, err
}

func TaskNameToQNameAndRKey(taskname string) (string, string) {
	l := strings.Split(taskname, ".")
	if len(l) == 3 {
		return fmt.Sprintf("%s.%s", l[0], l[1]), l[2]
	}
	return taskname, ""

}
