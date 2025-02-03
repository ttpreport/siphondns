package codec

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
)

type QtypeCodec struct {
	HexCmdEncoder

	PROLOGUE uint16
	EPILOGUE uint16
	OFFSET   uint16
}

func NewQtypeCodec() *QtypeCodec {
	codec := &QtypeCodec{
		PROLOGUE: 12345,
		EPILOGUE: 56789,
		OFFSET:   512,
	}

	return codec
}

func (codec *QtypeCodec) GetPrologue() uint16 {
	return codec.PROLOGUE
}

func (codec *QtypeCodec) GetEpilogue() uint16 {
	return codec.EPILOGUE
}

func (codec *QtypeCodec) Compress(data []byte) ([]byte, error) {
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

func (codec *QtypeCodec) Decompress(compressedData []byte) ([]byte, error) {
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

func (codec *QtypeCodec) Encode(data []byte) []uint16 {
	var result []uint16

	result = append(result, codec.PROLOGUE)

	compressed, _ := codec.Compress(data)

	length := len(compressed)
	var val uint16
	for i := 0; i < length; i += 2 {
		if i+1 < length {
			val = binary.LittleEndian.Uint16(compressed[i : i+2])
		} else {
			val = uint16(compressed[i])
		}

		result = append(result, val+codec.OFFSET)
	}

	result = append(result, codec.EPILOGUE)

	return result
}

func (codec *QtypeCodec) Decode(data []uint16) ([]byte, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("Message too short")
	}

	if !(data[0] == codec.PROLOGUE) {
		return nil, fmt.Errorf("Unexpected prologue: %v", data[0])
	}

	if !(data[len(data)-1] == codec.EPILOGUE) {
		return nil, fmt.Errorf("Unexpected epilogue: %v", data[len(data)])
	}

	msg := make([]byte, len(data[1:len(data)-1])*2)
	for i, val := range data[1 : len(data)-1] {
		binary.LittleEndian.PutUint16(msg[i*2:], val-codec.OFFSET)
	}

	return codec.Decompress(msg)
}
