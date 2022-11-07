package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/huo-ju/dfserver/pkg/data"
	dfpb "github.com/huo-ju/dfserver/pkg/pb"
	"github.com/huo-ju/dfserver/pkg/rabbitmq"
	"github.com/huo-ju/dfserver/pkg/service"
	"github.com/huo-ju/dfserver/pkg/worker"
	"google.golang.org/protobuf/proto"

	"github.com/pelletier/go-toml"
)

var GitCommit string
var cfg Config

type Config struct {
	Amqp_url         string
	Base_retry_delay uint
	Max_retries      uint
	Queue_qos        uint
	Queue            map[string]data.QueueItem
	Worker           map[string]data.WorkerItem
	Service          map[string]map[string]string

	Jwt_secret      string
	Tls_ca_cert     string
	Tls_client_cert string
	Tls_client_key  string
	Tls_servername  string
}

var discordservices map[string]*service.DiscordService

func loadtomlconf(configspath string, filename string) error {
	f, err := os.Open(fmt.Sprintf("%s/%s", configspath, filename))
	if err == nil {
		defer f.Close()
		data, err := ioutil.ReadAll(f)
		if err == nil {
			return toml.Unmarshal(data, &cfg)
		}
	}
	return err
}

func amqpQueueConnect(connectstr string, queues map[string]data.QueueItem, baseRetryDelay uint, maxRetries uint) *rabbitmq.AmqpQueue {
	amqpconfig := &rabbitmq.Config{Qos: 1}
	var tlscfg *tls.Config
	if cfg.Tls_ca_cert != "" && cfg.Tls_client_cert != "" && cfg.Tls_client_key != "" && strings.Index(cfg.Amqp_url, "amqps") == 0 {
		tlscfg = new(tls.Config)
		tlscfg.RootCAs = x509.NewCertPool()
		ca, err := ioutil.ReadFile(cfg.Tls_ca_cert)
		if err == nil {
			tlscfg.RootCAs.AppendCertsFromPEM(ca)
		} else {
			log.Fatal("ca load err: %s", err)
		}
		cert, err := tls.LoadX509KeyPair(cfg.Tls_client_cert, cfg.Tls_client_key)
		if err == nil {
			tlscfg.Certificates = append(tlscfg.Certificates, cert)
		} else {
			log.Fatal("x509keypair load err: %s", err)
		}
		if cfg.Tls_servername != "" {
			tlscfg.ServerName = cfg.Tls_servername
		}
	}

	amqpQueue, err := rabbitmq.Init(connectstr, baseRetryDelay, maxRetries, amqpconfig, tlscfg)
	for err != nil {
		log.Printf("Err amqpQueue %s\n", err)
		log.Println("wait 5 Second for reconnect resultamqp")
		time.Sleep(5 * time.Second)
		amqpQueue, err = rabbitmq.Init(connectstr, baseRetryDelay, maxRetries, amqpconfig, tlscfg)
	}
	log.Printf("amqp connected")
	return amqpQueue
}

func main() {
	var configpath string
	var conffilename string
	var port string

	quitch := make(chan os.Signal, 1)
	flag.StringVar(&configpath, "confpath", "/etc/dfbotserver", "configurate file path")
	flag.StringVar(&port, "port", ":2323", "http server port")
	flag.StringVar(&conffilename, "conf", "config.toml", "configurate file name")
	flag.Parse()
	loadtomlconf(configpath, conffilename)

	log.Printf("Version: %s", GitCommit)
	amqpQueue := amqpQueueConnect(cfg.Amqp_url, cfg.Queue, cfg.Base_retry_delay, cfg.Max_retries)
	defer amqpQueue.Close()

	err := amqpQueue.Declare(cfg.Queue)
	if err != nil {
		log.Fatalf("declare queue err %s\n", err)
	}

	discordservices = make(map[string]*service.DiscordService)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	StartService(ctx, cfg.Service, amqpQueue)
	StartWorker(ctx, cfg.Worker, amqpQueue)

	signal.Notify(quitch, os.Interrupt, os.Kill, syscall.SIGTERM)
	signalType := <-quitch
	signal.Stop(quitch)
	//cleanup before exit
	log.Println("Exit command received. Exiting...")
	log.Println("Signal type : ", signalType)
}

func StartWorker(ctx context.Context, wrkcfg map[string]data.WorkerItem, amqpQueue *rabbitmq.AmqpQueue) {
	//load all worker
	workerloader := worker.InitWorkerLoader(discordservices)

	for _, v := range wrkcfg {
		for _, bindkey := range v.Bindkeys {
			workerQueueMessageChannel, err := amqpQueue.Consume(strings.TrimSpace(v.Name), strings.TrimSpace(bindkey), 1)
			if err == nil {
				go func() {
					for d := range workerQueueMessageChannel {
						var task dfpb.Task
						err := proto.Unmarshal(d.Body, &task)
						if err == nil {
							inputtask := task.InputList[len(task.OutputList)]
							if len(task.OutputList) == 0 {
								//return error
							}
							n, workerkey := data.TaskNameToQNameAndRKey(inputtask.Name)
							wrk := workerloader.GetWorker(n, workerkey)
							if wrk == nil {
								log.Println("unsupported worker:", n)
								//TODO Nack?
								//d.Nack(false, true)
								continue
							}
							lastinputtask := task.InputList[len(task.OutputList)-1]
							canack, err := wrk.Work(task.OutputList, lastinputtask, inputtask.Settings)
							if err != nil {
								//TODO: response err
							}
							if canack == true {
								d.Ack(false)
							} else {
								d.Nack(false, true)
							}
						} else {
							//TODO: response err
							d.Ack(false)
						}
					}
					log.Println("routine channel closed, exit")
					return
				}()
			} else {
				log.Printf("amqp Consume queue %s key %s err: %s", strings.TrimSpace(v.Name), strings.TrimSpace(bindkey), err)
			}
		}
	}
}

func StartService(ctx context.Context, servicecfg map[string]map[string]string, amqpQueue *rabbitmq.AmqpQueue) {
	for _, v := range servicecfg {
		servicename := v["name"]
		sc := strings.Split(servicename, ".")
		if sc[0] == "discord" {
			discordToken := v["token"]
			discordPrefix := v["prefix"]
			newdiscordservice := service.NewDiscordService(servicename, discordToken, discordPrefix, amqpQueue)
			discordservices[servicename] = newdiscordservice
			go func() {
				err := discordservices[servicename].Start(ctx)
				if err != nil {
					log.Fatalln("StartService discord:", err)
				}
			}()
		} else if sc[0] == "grpc" {
			listen := v["listen"]
			grpcservice := service.NewGrpcService(servicename, listen, amqpQueue)
			go grpcservice.Start(ctx)
		}

	}
}
