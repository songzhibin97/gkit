package controller_redis

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"sync"
)

type reliableQueueKeys struct {
	ready        string
	inflight     string
	visibility   string
	outcomes     string
	repairCursor string
	prefix       string
}

var reliableSlotTags = struct {
	sync.RWMutex
	tags map[int]string
}{tags: make(map[int]string)}

func deriveReliableQueueKeys(queue string) reliableQueueKeys {
	tag, ok := redisHashTag(queue)
	if !ok {
		tag = reliableTagForSlot(redisClusterSlot(queue))
	}
	digest := sha256.Sum256([]byte(queue))
	prefix := fmt.Sprintf("{%s}:gkit:%x", tag, digest)
	return reliableQueueKeys{
		ready:        queue,
		inflight:     prefix + ":inflight",
		visibility:   prefix + ":visibility",
		outcomes:     prefix + ":ack-outcomes",
		repairCursor: prefix + ":repair-cursor",
		prefix:       prefix,
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
	if slot < 0 || slot >= 16384 {
		panic("controller_redis: invalid Redis Cluster slot")
	}
	reliableSlotTags.RLock()
	tag := reliableSlotTags.tags[slot]
	reliableSlotTags.RUnlock()
	if tag != "" {
		return tag
	}

	tag = findReliableTagForSlot(slot)
	reliableSlotTags.Lock()
	defer reliableSlotTags.Unlock()
	if cached := reliableSlotTags.tags[slot]; cached != "" {
		return cached
	}
	if len(reliableSlotTags.tags) >= 16384 {
		panic("controller_redis: reliable queue tag cache exhausted")
	}
	reliableSlotTags.tags[slot] = tag
	return tag
}

func findReliableTagForSlot(slot int) string {
	for candidate := 0; candidate <= 131071; candidate++ {
		tag := fmt.Sprintf("gkit-%x", candidate)
		if int(redisCRC16([]byte(tag))%16384) == slot {
			return tag
		}
	}
	panic("controller_redis: reliable queue tag search exhausted")
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
