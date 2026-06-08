//go:build windows

package tray

import (
	"bytes"
	"encoding/binary"
)

// makeIcon wraps the PNG in a valid ICO container (Vista+ PNG-frame format).
func makeIcon() []byte {
	pngData := makePNG()

	var buf bytes.Buffer
	// ICO header
	binary.Write(&buf, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // type = icon
	binary.Write(&buf, binary.LittleEndian, uint16(1)) // image count
	// Directory entry
	buf.WriteByte(32)                                                      // width
	buf.WriteByte(32)                                                      // height
	buf.WriteByte(0)                                                       // color count
	buf.WriteByte(0)                                                       // reserved
	binary.Write(&buf, binary.LittleEndian, uint16(1))                    // planes
	binary.Write(&buf, binary.LittleEndian, uint16(32))                   // bits per pixel
	binary.Write(&buf, binary.LittleEndian, uint32(len(pngData)))         // image size
	binary.Write(&buf, binary.LittleEndian, uint32(6+16))                 // data offset
	buf.Write(pngData)
	return buf.Bytes()
}
