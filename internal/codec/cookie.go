package codec

import (
	"bytes"
	"compress/zlib"
	"encoding/hex"
	"fmt"
	"io"
)

type CookieCodec struct {
	HexCmdEncoder

	PROLOGUE string
	EPILOGUE string
}

func NewCookieCodec() *CookieCodec {
	return &CookieCodec{
		PROLOGUE: "13371337133713371337133713371337",
		EPILOGUE: "73317331733173317331733173317331",
	}
}

func (codec *CookieCodec) GetPrologue() string {
	return codec.PROLOGUE
}

func (codec *CookieCodec) GetEpilogue() string {
	return codec.EPILOGUE
}

func (codec *CookieCodec) Compress(data []byte) ([]byte, error) {
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

func (codec *CookieCodec) Decompress(compressedData []byte) ([]byte, error) {
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

func (codec *CookieCodec) Encode(data []byte) []string {
	var result []string

	result = append(result, codec.PROLOGUE)

	compressed, _ := codec.Compress(data)
	msg := hex.EncodeToString(compressed)

	result = append(result, codec.chunkify(msg, len(codec.PROLOGUE))...)
	result = append(result, codec.EPILOGUE)

	return result
}

func (codec *CookieCodec) Decode(data []string) ([]byte, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("Message too short")
	}

	if !(data[0] == codec.PROLOGUE) {
		return nil, fmt.Errorf("Unexpected prologue: %v", data[0])
	}

	if !(data[len(data)-1] == codec.EPILOGUE) {
		return nil, fmt.Errorf("Unexpected epilogue: %v", data[len(data)])
	}

	var encodedMsg string

	for _, chunk := range data[1 : len(data)-1] {
		encodedMsg += chunk
	}

	msg, err := hex.DecodeString(encodedMsg)
	if err != nil {
		return nil, err
	}

	return codec.Decompress(msg)
}

func (codec *CookieCodec) chunkify(s string, chunkSize int) []string {
	var chunks []string
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}
