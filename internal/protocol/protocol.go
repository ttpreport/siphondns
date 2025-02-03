package protocol

type ProtocolClient interface {
	FetchCmd() (string, error)
	Send(data []byte) error
}

type ProtocolServer interface {
	Serve() error
	SendCmd(cmd string)
	FetchData() []byte
}
