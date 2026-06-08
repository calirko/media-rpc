package discord

import (
	"encoding/binary"
	"io"

	"github.com/gorilla/websocket"
)

// packetConn is the low-level transport abstraction (IPC socket or WebSocket).
type packetConn interface {
	sendPacket(op uint32, data []byte) error
	recvPacket() (op uint32, data []byte, err error)
	close()
}

// ipcConn wraps a stream (Unix socket or Windows named pipe).
type ipcConn struct{ r io.ReadWriteCloser }

func (c *ipcConn) sendPacket(op uint32, data []byte) error {
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:4], op)
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(data)))
	pkt := append(hdr, data...) //nolint:gocritic
	_, err := c.r.Write(pkt)
	return err
}

func (c *ipcConn) recvPacket() (uint32, []byte, error) {
	var hdr [8]byte
	if _, err := io.ReadFull(c.r, hdr[:]); err != nil {
		return 0, nil, err
	}
	op := binary.LittleEndian.Uint32(hdr[0:4])
	n := binary.LittleEndian.Uint32(hdr[4:8])
	payload := make([]byte, n)
	if _, err := io.ReadFull(c.r, payload); err != nil {
		return 0, nil, err
	}
	return op, payload, nil
}

func (c *ipcConn) close() { c.r.Close() }

// wsConn wraps a WebSocket connection (arpc protocol).
type wsConn struct{ c *websocket.Conn }

func (c *wsConn) sendPacket(op uint32, data []byte) error {
	pkt := make([]byte, 8+len(data))
	binary.LittleEndian.PutUint32(pkt[0:4], op)
	binary.LittleEndian.PutUint32(pkt[4:8], uint32(len(data)))
	copy(pkt[8:], data)
	return c.c.WriteMessage(websocket.BinaryMessage, pkt)
}

func (c *wsConn) recvPacket() (uint32, []byte, error) {
	_, msg, err := c.c.ReadMessage()
	if err != nil {
		return 0, nil, err
	}
	if len(msg) < 8 {
		return 0, msg, nil
	}
	op := binary.LittleEndian.Uint32(msg[0:4])
	return op, msg[8:], nil
}

func (c *wsConn) close() { c.c.Close() }
