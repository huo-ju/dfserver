package service

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/google/uuid"
	dfpb "github.com/huo-ju/dfserver/pkg/pb"
	"github.com/huo-ju/dfserver/pkg/rabbitmq"
	"google.golang.org/grpc"
)

type GrpcService struct {
	servicename string
	listen      string
	amqpQueue   *rabbitmq.AmqpQueue
}

type dfapiserver struct {
	dfpb.UnimplementedDfapiServer
}

func newDfapiServer() *dfapiserver {
	return &dfapiserver{}
}

func NewGrpcService(servicename string, listen string, amqpQueue *rabbitmq.AmqpQueue) *GrpcService {
	d := &GrpcService{servicename: servicename, listen: listen, amqpQueue: amqpQueue}
	return d
}

func (d *GrpcService) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", d.listen)
	if err != nil {
		return err
	}

	var opts []grpc.ServerOption
	grpcServer := grpc.NewServer(opts...)
	dfpb.RegisterDfapiServer(grpcServer, newDfapiServer())
	log.Printf("grpc is running.")
	grpcServer.Serve(lis)
	return nil
}

func (s *dfapiserver) RunTask(input *dfpb.Input, stream dfpb.Dfapi_RunTaskServer) error {
	log.Println("call grpc RunTask")
	fmt.Println(input)

	inputList := []*dfpb.Input{}
	outputList := []*dfpb.Output{}
	inputList = append(inputList, input)
	taskId := uuid.New().String()
	task := &dfpb.Task{TaskId: taskId, OutputList: outputList, InputList: inputList}
	log.Println("build task ")
	fmt.Println(task)

	output := &dfpb.Output{InputId: input.InputId}
	if err := stream.Send(output); err != nil {
		return err
	}
	return nil
}
