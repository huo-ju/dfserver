package worker

import (
	"strings"

	"github.com/huo-ju/dfserver/pkg/service"
)

type WorkerLoader struct {
	processworkers map[string]ProcessWorkerIface
}

func InitWorkerLoader(discordservice *service.DiscordService) *WorkerLoader {
	processworkers := make(map[string]ProcessWorkerIface)
	processworkers["process.file"] = &ProcessFileWorker{}
	processworkers["process.discord"] = &ProcessDiscordWorker{ds: discordservice}
	loader := &WorkerLoader{processworkers: processworkers}

	return loader
}

func (wl *WorkerLoader) GetWorker(name string) ProcessWorkerIface {
	if strings.HasPrefix(name, "process.") { //only support process worker
		worker, ok := wl.processworkers[name]
		if ok == false {
			return nil
		}
		return worker
	}
	return nil
}
