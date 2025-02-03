package codec

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
	"net"
)

type EcsCodec struct {
	HexCmdEncoder

	PROLOGUE net.IP
	EPILOGUE net.IP
	RESERVED []string
}

func NewEcsCodec() *EcsCodec {
	return &EcsCodec{
		PROLOGUE: net.ParseIP("13.3.7.0"),
		EPILOGUE: net.ParseIP("7.3.13.0"),
		RESERVED: []string{
			"0.0.0.0/8",
			"10.0.0.0/8",
			"100.64.0.0/10",
			"127.0.0.0/8",
			"169.254.0.0/16",
			"172.16.0.0/12",
			"192.0.0.0/24",
			"192.0.2.0/24",
			"192.88.99.0/24",
			"192.168.0.0/16",
			"198.18.0.0/15",
			"198.51.100.0/24",
			"203.0.113.0/24",
			"224.0.0.0/4",
			"233.252.0.0/24",
			"240.0.0.0/4",
			"255.255.255.255/32",
		},
	}
}

func (codec *EcsCodec) GetPrologue() net.IP {
	return codec.PROLOGUE
}

func (codec *EcsCodec) GetEpilogue() net.IP {
	return codec.EPILOGUE
}

func (codec *EcsCodec) isReserved(ip net.IP) bool {
	for _, addr := range codec.RESERVED {
		_, network, _ := net.ParseCIDR(addr)
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

func (codec *EcsCodec) Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer

	writer := zlib.NewWriter(&buf)
	_, err := writer.Write(data)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (codec *EcsCodec) Decompress(compressedData []byte) ([]byte, error) {
	buf := bytes.NewReader(compressedData)

	reader, err := zlib.NewReader(buf)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var result bytes.Buffer
	_, err = io.Copy(&result, reader)
	if err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

func (codec *EcsCodec) Encode(data []byte) []net.IP {
	var result []net.IP

	result = append(result, codec.PROLOGUE)

	compressed, _ := codec.Compress(data)
	msg := make([]byte, base64.StdEncoding.EncodedLen(len(compressed)))
	base64.StdEncoding.Encode(msg, compressed)

	var currentPart []byte
	for _, char := range msg {
		currentPart = append(currentPart, char)

		if len(currentPart) == 3 {
			encodedPart := net.IPv4(currentPart[0], currentPart[1], currentPart[2], byte(0))

			if codec.isReserved(encodedPart) {
				encodedPart = net.IPv4(byte(1), currentPart[0], currentPart[1], byte(0))
				result = append(result, encodedPart)

				encodedPart = net.IPv4(byte(1), currentPart[2], byte(0), byte(0))
				result = append(result, encodedPart)
			} else {
				result = append(result, encodedPart)
			}

			currentPart = nil
		}
	}

	for _, part := range currentPart {
		encodedPart := net.IPv4(byte(1), part, byte(0), byte(0))
		result = append(result, encodedPart)
	}

	result = append(result, codec.EPILOGUE)

	return result
}

func (codec *EcsCodec) Decode(data []net.IP) ([]byte, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("Message too short")
	}

	var encodedMsg []byte

	if !data[0].Equal(codec.PROLOGUE) {
		return nil, fmt.Errorf("Unexpected prologue: %v", data[0])
	}

	if !data[len(data)-1].Equal(codec.EPILOGUE) {
		return nil, fmt.Errorf("Unexpected epilogue: %v", data[len(data)])
	}

	for _, part := range data[1 : len(data)-1] {
		for _, char := range part.To4() {
			if char != byte(0) && char != byte(1) {
				encodedMsg = append(encodedMsg, char)
			}
		}
	}

	msg := make([]byte, base64.StdEncoding.DecodedLen(len(encodedMsg)))
	_, err := base64.StdEncoding.Decode(msg, encodedMsg)
	if err != nil {
		return nil, err
	}

	return codec.Decompress(msg)
}
