package generator

import (
	"errors"
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

// Settings: 配置
type Settings struct {
	// StartTime: 起始时间
	StartTime time.Time

	// NodeID: 服务器ID
	NodeID uint16
}

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

// NextID: 获取下一个ID
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

// timeUnit: 时间单位,纳秒
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

// Decompose: 根据id分解为mapping包含这组数据的所有信息
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

// NewSnowflake: 初始化
func NewSnowflake(st Settings) *Snowflake {
	sf := new(Snowflake)
	sf.sequence = uint16(1<<SequenceBit - 1)

	if st.StartTime.After(time.Now()) {
		return nil
	}
	if st.StartTime.IsZero() {
		sf.startTime = toSnowflakeTime(time.Date(2014, 9, 1, 0, 0, 0, 0, time.UTC))
	} else {
		sf.startTime = toSnowflakeTime(st.StartTime)
	}
	return sf
}
