package impl

import (
	"fmt"

	//"github.com/contiv/vpp/plugins/controller"
	"mocknet/plugins/server/rpctest"
	//"go.ligato.io/vpp-agent/v3/plugins/kvscheduler"
	"golang.org/x/net/context"
)

type Server struct {
	routeNotes []*rpctest.Message
	DataChan   chan rpctest.Message
}

func (receiver *Server) Process(ctx context.Context, message *rpctest.Message) (*rpctest.Message, error) {
	fmt.Println("server receive a message")
	//fmt.Println(message)
	//fmt.Println(message.Type)
	receiver.DataChan <- *message
	return &rpctest.Message{
		Type: 0,
	}, nil
}
