package codec

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"fmt"
	"io"
)

type DauCodec struct {
	HexCmdEncoder

	PROLOGUE []uint8
	EPILOGUE []uint8
}

func NewDauCodec() *DauCodec {
	codec := &DauCodec{
		PROLOGUE: []uint8{1, 3, 3, 7, 1, 3, 3, 7},
		EPILOGUE: []uint8{7, 3, 3, 1, 7, 3, 3, 1},
	}

	return codec
}

func (codec *DauCodec) GetPrologue() []uint8 {
	return codec.PROLOGUE
}

func (codec *DauCodec) GetEpilogue() []uint8 {
	return codec.EPILOGUE
}

func (codec *DauCodec) Compress(data []byte) ([]byte, error) {
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

func (codec *DauCodec) Decompress(compressedData []byte) ([]byte, error) {
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

func (codec *DauCodec) Encode(data []byte) [][]uint8 {
	var result [][]uint8

	result = append(result, codec.PROLOGUE)

	compressed, _ := codec.Compress(data)
	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(compressed)))
	base64.StdEncoding.Encode(encoded, compressed)

	for _, chunk := range codec.chunkify(encoded, 64) {
		result = append(result, chunk)
	}

	result = append(result, codec.EPILOGUE)

	return result
}

func (codec *DauCodec) Decode(data [][]uint8) ([]byte, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("Message too short")
	}

	if !bytes.Equal(data[0], codec.PROLOGUE) {
		return nil, fmt.Errorf("Unexpected prologue: %v", data[0])
	}

	if !bytes.Equal(data[len(data)-1], codec.EPILOGUE) {
		return nil, fmt.Errorf("Unexpected epilogue: %v", data[len(data)])
	}

	var encoded []uint8
	for _, chunk := range data[1 : len(data)-1] {
		encoded = append(encoded, chunk...)
	}

	msg := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	_, err := base64.StdEncoding.Decode(msg, encoded)
	if err != nil {
		return nil, err
	}

	return codec.Decompress(msg)
}

func (codec *DauCodec) chunkify(s []byte, chunkSize int) [][]byte {
	if chunkSize <= 0 {
		return nil
	}

	var chunks [][]byte
	for i := 0; i < len(s); i += chunkSize {
		end := i + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}
