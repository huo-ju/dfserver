package worker

import (
	"fmt"
	"strings"

	"github.com/huo-ju/dfserver/pkg/service"
)

type WorkerLoader struct {
	processworkers map[string]ProcessWorkerIface
}

func InitWorkerLoader(discordservices map[string]*service.DiscordService) *WorkerLoader {
	processworkers := make(map[string]ProcessWorkerIface)
	//processworkers["process.file"] = &ProcessFileWorker{}
	for servicekey, discordservice := range discordservices {
		processworkers["process."+servicekey] = &ProcessDiscordWorker{ds: discordservice}
	}
	loader := &WorkerLoader{processworkers: processworkers}

	return loader
}

func (wl *WorkerLoader) GetWorker(name string, servicekey string) ProcessWorkerIface {
	if strings.HasPrefix(name, "process.") { //only support process worker
		workername := fmt.Sprintf("%s.%s", name, servicekey)
		worker, ok := wl.processworkers[workername]
		if ok == false {
			return nil
		}
		return worker
	}
	return nil
}
