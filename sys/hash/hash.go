package hash

import "hash/crc32"

func Uint16(s string, table *crc32.Table) uint16 {
	return uint16(crc32.Checksum([]byte(s), table))
}

func Uint32(s string, table *crc32.Table) uint32 {
	return crc32.Checksum([]byte(s), table)
}

func Uint16IEEE(s string) uint16 {
	return uint16(crc32.ChecksumIEEE([]byte(s)))
}

func Uint32IEEE(s string) uint32 {
	return crc32.ChecksumIEEE([]byte(s))
}
