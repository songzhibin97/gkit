package file_helper

import (
	"encoding/binary"
	"errors"
)

var ErrInvalidPos = errors.New("invalid pos")

type Pos interface {
	Encode(start uint64, end uint64) []byte
	Decode(bs []byte) (uint64, uint64, error)
	Index(int) int // seek offset
}

func NewPos(posType string) Pos {
	switch posType {
	case "16":
		return Pos16{}
	case "32":
		return Pos32{}
	case "64":
		return Pos64{}
	default:
		return Pos64{}
	}
}

type Pos64 struct {
}

func (p Pos64) Encode(offset uint64, length uint64) []byte {
	pos := make([]byte, 16)
	binary.BigEndian.PutUint64(pos[:], offset)
	binary.BigEndian.PutUint64(pos[8:], length)
	return pos
}

func (p Pos64) Decode(bs []byte) (uint64, uint64, error) {
	if len(bs) != 16 {
		return 0, 0, ErrInvalidPos
	}
	return binary.BigEndian.Uint64(bs[:]), binary.BigEndian.Uint64(bs[8:]), nil
}

func (p Pos64) Index(i int) int {
	return i * 16
}

type Pos32 struct {
}

func (p Pos32) Encode(offset uint64, length uint64) []byte {
	pos := make([]byte, 8)
	binary.BigEndian.PutUint32(pos[:], uint32(offset))
	binary.BigEndian.PutUint32(pos[4:], uint32(length))
	return pos
}

func (p Pos32) Decode(bs []byte) (uint64, uint64, error) {
	if len(bs) != 8 {
		return 0, 0, ErrInvalidPos
	}
	return uint64(binary.BigEndian.Uint32(bs[:])), uint64(binary.BigEndian.Uint32(bs[4:])), nil
}

func (p Pos32) Index(i int) int {
	return i * 8
}

type Pos16 struct {
}

func (p Pos16) Encode(offset uint64, length uint64) []byte {
	pos := make([]byte, 4)
	binary.BigEndian.PutUint16(pos[:], uint16(offset))
	binary.BigEndian.PutUint16(pos[2:], uint16(length))
	return pos
}

func (p Pos16) Decode(bs []byte) (uint64, uint64, error) {
	if len(bs) != 4 {
		return 0, 0, ErrInvalidPos
	}
	return uint64(binary.BigEndian.Uint16(bs[:])), uint64(binary.BigEndian.Uint16(bs[2:])), nil
}

func (p Pos16) Index(i int) int {
	return i * 4
}
