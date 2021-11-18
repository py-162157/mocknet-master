package impl

import (
	"mocknet/plugins/server/rpctest"

	"golang.org/x/net/context"
)

type Server struct {
	routeNotes []*rpctest.Message
	DataChan   chan rpctest.Message
}

func (receiver *Server) Process(ctx context.Context, message *rpctest.Message) (*rpctest.Message, error) {
	//fmt.Println("server receive a message")
	//fmt.Println(message)
	//fmt.Println(message.Type)
	receiver.DataChan <- *message
	return &rpctest.Message{
		Type: 0,
	}, nil
}
