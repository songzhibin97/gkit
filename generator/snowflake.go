package generator

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

/*
Total: 			64bit
First: 			1bit
TimeBit:		39bit
NodeID: 		16bit(uint16)
SequenceID: 	8bit(uint8)
*/

const (
	TimeBit     = 39
	NodeBit     = 16
	SequenceBit = 8
)

// Snowflake is a distributed unique ID generator.
type Snowflake struct {
	// 互斥锁
	sync.Mutex

	// startTime: 开始时间
	startTime int64

	// elapsedTime: 经过的时间
	elapsedTime int64

	// sequence: 流水号
	sequence uint16

	// node: 节点
	node uint16
}

// NextID 获取下一个ID
func (s *Snowflake) NextID() (uint64, error) {
	const maskSequence = uint16(1<<SequenceBit - 1)

	s.Lock()
	defer s.Unlock()

	current := currentElapsedTime(s.startTime)
	if s.elapsedTime < current {
		s.elapsedTime = current
		s.sequence = 0
	} else {
		// s.elapsedTime >= current
		s.sequence = (s.sequence + 1) & maskSequence
		if s.sequence == 0 {
			s.elapsedTime++
			overtime := s.elapsedTime - current
			time.Sleep(sleepTime(overtime))
		}
	}
	return s.toID()
}

// timeUnit 时间单位,纳秒
const timeUnit = 1e7

func toSnowflakeTime(t time.Time) int64 {
	return t.UTC().UnixNano() / timeUnit
}

func currentElapsedTime(startTime int64) int64 {
	return toSnowflakeTime(time.Now()) - startTime
}

func sleepTime(overtime int64) time.Duration {
	return time.Duration(overtime)*10*time.Millisecond -
		time.Duration(time.Now().UTC().UnixNano()%timeUnit)*time.Nanosecond
}

func (s *Snowflake) toID() (uint64, error) {
	if s.elapsedTime >= 1<<TimeBit {
		return 0, errors.New("over the time limit")
	}

	return uint64(s.elapsedTime)<<(SequenceBit+NodeBit) |
		uint64(s.sequence)<<NodeBit |
		uint64(s.node), nil
}

// Decompose 根据id分解为mapping包含这组数据的所有信息
func Decompose(id uint64) map[string]uint64 {
	const maskSequence = uint64((1<<SequenceBit - 1) << NodeBit)
	const maskMachineID = uint64(1<<NodeBit - 1)

	msb := id >> 63
	t := id >> (SequenceBit + NodeBit)
	sequence := id & maskSequence >> NodeBit
	node := id & maskMachineID
	return map[string]uint64{
		"id":       id,
		"msb":      msb,
		"time":     t,
		"sequence": sequence,
		"node":     node,
	}
}

// localIPv4 获取本地IP
func localIPv4() (net.IP, error) {
	as, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, a := range as {
		i, ok := a.(*net.IPNet)
		if !ok || i.IP.IsLoopback() {
			continue
		}

		ip := i.IP.To4()
		if isPrivateIPv4(ip) {
			return ip, nil
		}
	}
	return nil, errors.New("no private ip address")
}

// isPrivateIPv4 判断是否有效IP
func isPrivateIPv4(ip net.IP) bool {
	return ip != nil &&
		(ip[0] == 10 || ip[0] == 172 && (ip[1] >= 16 && ip[1] < 32) || ip[0] == 192 && ip[1] == 168)
}

// IpToUint16 将IP地址转化为uint16
func IpToUint16(ip net.IP) (uint16, error) {
	return uint16(ip[2])<<8 + uint16(ip[3]), nil
}

// LocalIpToUint16 本地IP转化为uint16
func LocalIpToUint16() (uint16, error) {
	ip, err := localIPv4()
	if err != nil {
		return 0, err
	}
	fmt.Println(ip)
	return uint16(ip[2])<<8 + uint16(ip[3]), nil
}

// NewSnowflake 初始化
// StartTime 起始时间
// NodeID 服务器ID
func NewSnowflake(startTime time.Time, nodeID uint16) Generator {
	sf := new(Snowflake)
	sf.sequence = uint16(1<<SequenceBit - 1)
	sf.node = nodeID
	if startTime.After(time.Now()) {
		return nil
	}
	if startTime.IsZero() {
		sf.startTime = toSnowflakeTime(time.Date(2014, 9, 1, 0, 0, 0, 0, time.UTC))
	} else {
		sf.startTime = toSnowflakeTime(startTime)
	}
	return sf
}
