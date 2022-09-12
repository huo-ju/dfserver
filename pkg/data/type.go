package data

type UserInfo struct {
	Provider       string `json:"provider"`
	ProviderUserId string `json:"provideruserid"`
	ProviderData   string `json:providerdata"`
}

type DiffSettings struct {
	Prompt            string  `json:"prompt"`
	Seed              uint64  `json:"seed"`
	Number            uint    `json:"number"`
	Height            uint    `json:"height"`
	Width             uint    `json:"width"`
	NumInferenceSteps uint    `json:"num_inference_steps"`
	Guidance_scale    float32 `json:"guidance_scale"`
	Eta               float32 `json:"eta"`
}

type AISettings struct {
	AiName   string      `json:"ainame"`
	Settings interface{} `json:"settings"`
}

type InputTask struct {
	Id         string      `json:"id"`
	User       *UserInfo   `json:"user"`
	AiSettings *AISettings `json:"aisettings"`
}

type QueueItem struct {
	Name     string
	Bindkeys []string
}

type WorkerItem struct {
	Name    string
	Bindkey string
}
