package exporter

import (
	"bytes"
	"encoding/binary"
)

// writeBE16 appends v to b as a big-endian uint16. Writing to a
// bytes.Buffer cannot fail, so no error is returned — this keeps the
// binary ASE/ACO encoders free of unreachable error checks.
func writeBE16(b *bytes.Buffer, v uint16) {
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], v)
	b.Write(tmp[:])
}

// writeBE32 appends v to b as a big-endian uint32. See writeBE16.
func writeBE32(b *bytes.Buffer, v uint32) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], v)
	b.Write(tmp[:])
}
