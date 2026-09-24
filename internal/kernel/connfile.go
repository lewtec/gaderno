package kernel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// ConnectionFile is the Jupyter connection file JSON.
type ConnectionFile struct {
	ShellPort       int    `json:"shell_port"`
	IOPubPort       int    `json:"iopub_port"`
	StdinPort       int    `json:"stdin_port"`
	ControlPort     int    `json:"control_port"`
	HBPort          int    `json:"hb_port"`
	IP              string `json:"ip"`
	Key             string `json:"key"`
	Transport       string `json:"transport"`
	SignatureScheme string `json:"signature_scheme"`
	KernelName      string `json:"kernel_name"`
}

// KeyBytes returns the HMAC key.
func (c ConnectionFile) KeyBytes() []byte {
	return []byte(c.Key)
}

// Endpoint returns tcp://ip:port for a channel port.
func (c ConnectionFile) Endpoint(port int) string {
	return fmt.Sprintf("%s://%s:%d", c.Transport, c.IP, port)
}

// NewConnectionFile allocates free ports and a random key.
func NewConnectionFile(kernelName string) (ConnectionFile, error) {
	ports := make([]int, 5)
	for i := range ports {
		p, err := freePort()
		if err != nil {
			return ConnectionFile{}, err
		}
		ports[i] = p
	}
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return ConnectionFile{}, err
	}
	return ConnectionFile{
		ShellPort:       ports[0],
		IOPubPort:       ports[1],
		StdinPort:       ports[2],
		ControlPort:     ports[3],
		HBPort:          ports[4],
		IP:              "127.0.0.1",
		Key:             hex.EncodeToString(key),
		Transport:       "tcp",
		SignatureScheme: "hmac-sha256",
		KernelName:      kernelName,
	}, nil
}

// WriteConnectionFile writes JSON with mode 0600 to dir, returns path.
func WriteConnectionFile(dir string, cf ConnectionFile) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	raw, err := json.MarshalIndent(cf, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, "connection.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return "", err
	}
	return path, nil
}

func freePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port, nil
}
