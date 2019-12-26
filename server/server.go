package server

import (
	"fmt"
	"io"
	"log"
	"net"

	"github.com/go-redis/redis/v7"
	"github.com/muchiko/go_grpc_redis/pb"
	"google.golang.org/grpc"
)

func Run() {
	lis, err := net.Listen("tcp", ":50080")
	if err != nil {
		log.Fatal(err)
	}
	defer lis.Close()

	client := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	pong, err := client.Ping().Result()
	fmt.Println(pong, err)

	s := grpc.NewServer()
	pb.RegisterSocketServiceServer(s, &server{
		rdb: client,
	})
	err = s.Serve(lis)
	if err != nil {
		log.Fatal(err)
	}
}

type server struct {
	rdb *redis.Client
}

func (s *server) Transport(stream pb.SocketService_TransportServer) error {
	for {
		in, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		fmt.Println(in.Message)

		// call
		switch in.Message {
		case "set":
			err := s.rdb.Set("key", in.Message, 0).Err()
			if err != nil {
				fmt.Println(err)
				return err
			}
		case "sub":
			pubsub := s.rdb.Subscribe("room")
			_, err := pubsub.Receive()
			if err != nil {
				fmt.Println(err)
				return err
			}
			ch := pubsub.Channel()

			go func(ch <-chan *redis.Message, stream pb.SocketService_TransportServer) {
				for {
					select {
					case msg := <-ch:
						fmt.Println("Transport")
						stream.Send(&pb.Payload{
							Message: msg.Payload,
						})
					}
				}
			}(ch, stream)
		case "pub":
			err := s.rdb.Publish("room", "in-api").Err()
			if err != nil {
				fmt.Println(err)
				return err
			}
		default:
			return nil
		}

		err = stream.Send(&pb.Payload{
			Message: "OK",
		})
		if err != nil {
			fmt.Println(err)
			return err
		}
	}
}
