package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/kek/pek/internal/protocol"
)

func main() {
	method := flag.String("method", "ecs", "channel type [ecs, cookie, qtype, dau]")
	flag.Parse()

	var proto protocol.ProtocolServer

	switch *method {
	case "ecs":
		proto = protocol.NewEcsProtoServer()
	case "cookie":
		proto = protocol.NewCookieProtoServer()
	case "qtype":
		proto = protocol.NewQtypeProtoServer()
	case "dau":
		proto = protocol.NewDauProtoServer()
	default:
		fmt.Printf("-method can only be one of [ecs, cookie, qtype, dau]")
		return
	}

	fmt.Println("Starting DNS server")

	go func() {
		err := proto.Serve()
		if err != nil {
			panic(fmt.Sprintf("Failed to start server: %s\n", err.Error()))
		}
	}()

	for {
		scanner := bufio.NewScanner(os.Stdin)
		fmt.Print("cmd> ")

		if scanner.Scan() {
			input := scanner.Text()
			proto.SendCmd(input)
		}

		response := proto.FetchData()
		fmt.Printf("Response:\n %s\n", string(response))
	}
}
