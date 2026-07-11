package controller_redis

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
)

type reliableQueueKeys struct {
	ready      string
	inflight   string
	visibility string
	outcomes   string
	prefix     string
}

var reliableSlotTags struct {
	sync.Once
	tags [16384]string
}

func deriveReliableQueueKeys(queue string) reliableQueueKeys {
	tag, ok := redisHashTag(queue)
	if !ok {
		tag = reliableTagForSlot(redisClusterSlot(queue))
	}
	digest := sha256.Sum256([]byte(queue))
	prefix := fmt.Sprintf("{%s}:gkit:%x", tag, digest)
	return reliableQueueKeys{
		ready:      queue,
		inflight:   prefix + ":inflight",
		visibility: prefix + ":visibility",
		outcomes:   prefix + ":ack-outcomes",
		prefix:     prefix,
	}
}

func redisHashTag(key string) (string, bool) {
	open := strings.IndexByte(key, '{')
	if open < 0 || open+1 >= len(key) {
		return "", false
	}
	closeOffset := strings.IndexByte(key[open+1:], '}')
	if closeOffset <= 0 {
		return "", false
	}
	return key[open+1 : open+1+closeOffset], true
}

func redisClusterSlot(key string) int {
	if tag, ok := redisHashTag(key); ok {
		key = tag
	}
	return int(redisCRC16([]byte(key)) % 16384)
}

func reliableTagForSlot(slot int) string {
	reliableSlotTags.Do(func() {
		remaining := len(reliableSlotTags.tags)
		for candidate := 0; candidate <= 131071 && remaining > 0; candidate++ {
			tag := fmt.Sprintf("gkit-%x", candidate)
			candidateSlot := int(redisCRC16([]byte(tag)) % 16384)
			if reliableSlotTags.tags[candidateSlot] == "" {
				reliableSlotTags.tags[candidateSlot] = tag
				remaining--
			}
		}
		if remaining != 0 {
			panic("controller_redis: reliable queue tag search exhausted")
		}
	})
	if slot < 0 || slot >= len(reliableSlotTags.tags) {
		panic("controller_redis: invalid Redis Cluster slot")
	}
	return reliableSlotTags.tags[slot]
}

// redisCRC16 implements the CRC-16/XMODEM variant used by Redis Cluster.
func redisCRC16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b) << 8
		for bit := 0; bit < 8; bit++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
