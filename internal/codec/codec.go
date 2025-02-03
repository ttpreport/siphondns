package codec

import (
	"encoding/hex"
	"fmt"
)

type Codec[T any] interface {
	Compress(data []byte) ([]byte, error)
	Decompress(compressedData []byte) ([]byte, error)
	Encode(data []byte) []T
	Decode(data []T) ([]byte, error)
	EncodeCmd(data string) string
	DecodeCmd(data string) (string, error)
	GetEpilogue() T
	GetPrologue() T
}

type HexCmdEncoder struct{}

func (*HexCmdEncoder) EncodeCmd(data string) string {
	return fmt.Sprintf("%s.", hex.EncodeToString([]byte(data)))
}

func (*HexCmdEncoder) DecodeCmd(data string) (string, error) {
	decoded, err := hex.DecodeString(data[:len(data)-1])
	return string(decoded), err
}
