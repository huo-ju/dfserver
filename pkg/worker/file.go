package worker

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/google/uuid"

	dfpb "github.com/huo-ju/dfserver/pkg/pb"
)

type ProcessFileWorker struct {
}

func (f *ProcessFileWorker) Name() string {
	return "process.file"
}

func mimeTofileext(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg", "image/jpg":
		return ".jpg"
	}
	return ""

}
func (f *ProcessFileWorker) Work(outputList []*dfpb.Output, lastinput *dfpb.Input, settingsdata []byte) (bool, error) {
	lastoutput := outputList[len(outputList)-1]
	var settings map[string]interface{}
	err := json.Unmarshal(settingsdata, &settings)
	if err != nil {
		//TODO: save err log
		return true, err
	}
	filename, ok := settings["filename"]
	if ok == false {
		filename = uuid.New().String()
	}
	ext := mimeTofileext(*lastoutput.MimeType)
	err = os.WriteFile(fmt.Sprintf("output-%s%s", filename, ext), lastoutput.Data, 0644)
	//TODO: save err log
	return true, err
}
