package api

import (
	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/huo-ju/dfserver/pkg/data"

	"github.com/labstack/echo"
)

type CustomBinder struct{}

func (cb *CustomBinder) Bind(i interface{}, c echo.Context) (err error) {
	db := new(echo.DefaultBinder)
	switch i.(type) {
	case *[]data.InputTask:
		bodyBytes, err := ioutil.ReadAll(c.Request().Body)
		json.Unmarshal(bodyBytes, &i)
		inputtasks, ok := i.(*[]data.InputTask)
		if ok == false {
			return errors.New("can't parse the input task list")
		}
		for i := 0; i < len(*inputtasks); i++ {
			switch (*inputtasks)[i].AiSettings.AiName {
			case "sd14":
				settings, ok := (*inputtasks)[i].AiSettings.Settings.(map[string]interface{})
				if ok == false {
					return errors.New("can't parse the AI Settings")
				}

				dfsettings := &data.DiffSettings{}
				for k, v := range settings {
					switch k {
					case "prompt":
						dfsettings.Prompt = v.(string)
					case "height":
						iv, ok := v.(float64)
						if ok == false {
							return errors.New("height should be a number")
						}
						dfsettings.Height = uint(iv)
					case "width":
						iv, ok := v.(float64)
						if ok == false {
							return errors.New("width should be a unsigned int")
						}
						dfsettings.Width = uint(iv)
					case "num_inference_steps":
						iv, ok := v.(float64)
						if ok == false {
							return errors.New("num_inference_steps should be a unsigned int")
						}
						dfsettings.NumInferenceSteps = uint(iv)
					case "guidance_scale":
						iv, ok := v.(float64)
						if ok == false {
							return errors.New("guidance_scale should be a float")
						}
						dfsettings.Guidance_scale = float32(iv)
					case "eta":
						iv, ok := v.(float64)
						if ok == false {
							return errors.New("eta should be a float")
						}
						dfsettings.Eta = float32(iv)
					}
				}
				(*inputtasks)[i].AiSettings.Settings = dfsettings
				return nil
			default:
				return errors.New("unsupported AI name")
			}
		}
		return err
	default:
		if err = db.Bind(i, c); err != echo.ErrUnsupportedMediaType {
			return
		}
		return err
	}
}
