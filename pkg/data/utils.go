package data

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/google/uuid"
	dfpb "github.com/huo-ju/dfserver/pkg/pb"
)

var ArgsList []string = []string{
	"-h", "--help", "--tokenize", "-t", "--height", "-H", "--width", "-W",
	"--cfg_scale", "-C", "--number", "-n", "--separate-images", "-i", "--grid", "-g",
	"--strength", "-E", "--init_image_url", "-IMG",
	"--sampler", "-A", "--steps", "-s", "--seed", "-S", "--prior", "-p", "--upscale", "-U", "--face",
	"--model", "-M"}

const (
	max_steps  = 150
	max_width  = 1024
	max_height = 1024
)

func CreateImageUpscaleInputTask(data *[]byte) *dfpb.Input {
	upscaleainame := "ai.realesrgan"
	inputId := uuid.New().String()
	input := &dfpb.Input{InputId: inputId}

	//ModelType
	upscalesettings := &RealEsrganSettings{}
	upscalesettings.ModelType = "general"

	settingstr, _ := json.Marshal(upscalesettings)
	input.Name = upscaleainame
	input.Settings = settingstr
	if data != nil {
		input.Data = *data

	}
	return input
}

func CreateInterrogatorInputTask(data *[]byte) *dfpb.Input {
	ainame := "ai.clipinterrogator"
	inputId := uuid.New().String()
	input := &dfpb.Input{InputId: inputId}

	//ModelType
	input.Name = ainame
	input.Settings = []byte("{}")
	if data != nil {
		input.Data = *data

	}
	return input
}

func CreateTask(il []*dfpb.Input, ol []*dfpb.Output) *dfpb.Task {
	var outputlist []*dfpb.Output
	if ol == nil {
		outputlist = []*dfpb.Output{}
	} else {
		outputlist = ol
	}

	taskId := uuid.New().String()
	task := &dfpb.Task{TaskId: taskId, OutputList: outputlist, InputList: il}
	return task
}

func CreateSDTaskWithCmdArgs(args *CommandArgs, imgbuff *bytes.Buffer, initimgurl string, removeseed bool) (*dfpb.Task, string) {
	publishkey := "all"

	//create task input and outputlist
	inputList := []*dfpb.Input{}
	outputList := []*dfpb.Output{}

	settings := &DiffSettings{Pipeline: "text2img", InitImageUrl: initimgurl}

	ainame := ""
	if strings.HasPrefix(args.Cmd, "!dream ") {
		ainame = "ai.sd14"
	}

	upscale := false

	settings.Prompt = strings.Replace(args.Cmd, "!dream ", "", 1)
	for _, a := range args.Args {
		item := strings.Split(a, " ")
		switch item[0] {
		case "--height", "-H":
			v, err := strconv.ParseUint(item[1], 10, 32)
			if err == nil {
				if uint(v) > max_height {
					v = 512 //set default value
				}

				settings.Height = uint(v)
			}
		case "--width", "-W":
			v, err := strconv.ParseUint(item[1], 10, 32)
			if err == nil {
				if uint(v) > max_width {
					v = 512 //set default value
				}
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
				if uint(v) > max_steps {
					v = 50 //set default value
				}
				settings.NumInferenceSteps = uint(v)
			}
		case "--strength", "-E":
			v, err := strconv.ParseFloat(item[1], 32)
			if err == nil {
				settings.Strength = float32(v)
			}
		case "--init_image_url", "-IMG":
			if settings.InitImageUrl == "" {
				settings.InitImageUrl = item[1]
			}

		case "--seed", "-S":
			if removeseed == true {
				continue
			}
			v, err := strconv.ParseUint(item[1], 10, 64)
			if err == nil {
				settings.Seed = uint64(v)
			}
		case "--cfg_scale", "-C":
			v, err := strconv.ParseFloat(item[1], 32)
			if err == nil {
				settings.Guidance_scale = float32(v)
			}
		case "--upscale", "-U", "--face":
			upscale = true
		case "--model", "-M":
			publishkey = item[1]
		}
	}

	inputId := uuid.New().String()
	input := &dfpb.Input{InputId: inputId}

	if imgbuff != nil {
		input.Data = imgbuff.Bytes()
		settings.Pipeline = "img2img"
	} else if settings.InitImageUrl != "" {
		buff, err := DownloadFile(settings.InitImageUrl)
		if err == nil {
			input.Data = buff.Bytes()
			settings.Pipeline = "img2img"
		}
	}

	settingstr, _ := json.Marshal(settings)
	input.Name = ainame
	input.Settings = settingstr
	inputList = append(inputList, input)

	if upscale == true {
		upscaleainame := "ai.realesrgan"
		inputId := uuid.New().String()
		input := &dfpb.Input{InputId: inputId}

		//ModelType
		upscalesettings := &RealEsrganSettings{}
		for _, a := range args.Args {
			item := strings.Split(a, " ")
			switch item[0] {
			case "--upscale", "-U":
				if len(item) == 1 {
					upscalesettings.ModelType = "general"
				} else {
					if item[1] == "g" {
						upscalesettings.ModelType = "general"
					} else if item[1] == "a" {
						upscalesettings.ModelType = "anime"
					}

				}
			case "--face":
				upscalesettings.FaceEnhance = true
			}
		}

		settingstr, _ := json.Marshal(upscalesettings)
		input.Name = upscaleainame
		input.Settings = settingstr
		inputList = append(inputList, input)
	}

	taskId := uuid.New().String()
	task := &dfpb.Task{TaskId: taskId, OutputList: outputList, InputList: inputList}
	return task, publishkey
}

func CreateGptNeoTask(prompt string) *dfpb.Task {
	inputList := []*dfpb.Input{}
	outputList := []*dfpb.Output{}
	if strings.HasPrefix(prompt, "!build ") {
		prompt = prompt[len("!build "):]
	}
	ainame := "ai.gptneo"

	settings := &GptNeoSettings{}
	settings.Prompt = prompt

	inputId := uuid.New().String()
	input := &dfpb.Input{InputId: inputId}
	settingstr, _ := json.Marshal(settings)
	input.Name = ainame
	input.Settings = settingstr
	inputList = append(inputList, input)
	taskId := uuid.New().String()
	task := &dfpb.Task{TaskId: taskId, OutputList: outputList, InputList: inputList}
	return task
}

func AddDiscordInputTask(name string, reference *discordgo.MessageReference, task *dfpb.Task) {
	inputId := uuid.New().String()
	input := &dfpb.Input{InputId: inputId}
	settingstr, _ := json.Marshal(reference)
	input.Settings = settingstr
	input.Name = name
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
				input := &dfpb.Input{InputId: inputId}
				settingstr, ok := mv["settings"].(string)
				if ok == true {
					input.Settings = []byte(settingstr)
				}

				namestr, ok := mv["name"].(string)
				if ok == true {
					input.Name = namestr
				}

				inputList = append(inputList, input)
			} else {
				return nil, errors.New("can't parse input data")
			}
		}
		task.InputList = inputList
		taskId := uuid.New().String()
		task.TaskId = taskId
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

func DownloadFile(url string) (*bytes.Buffer, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.ContentLength <= 2097152 {
		var buf bytes.Buffer
		_, err = io.Copy(&buf, resp.Body)
		return &buf, err
	} else {
		return nil, errors.New("Error: the  maximum supported image size is 2MB")
	}

}
