package arp

import (
	"errors"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

type timeoutPacketReader struct {
	reads int
}

func (r *timeoutPacketReader) ReadPacketData() ([]byte, gopacket.CaptureInfo, error) {
	r.reads++
	return nil, gopacket.CaptureInfo{}, pcap.NextErrorTimeoutExpired
}

func TestOpenARPHandleUsesFiniteReadTimeout(t *testing.T) {
	sentinel := errors.New("stop after observing options")
	var gotTimeout time.Duration

	_, err := openARPHandle("eth-test", func(name string, snaplen int32, promisc bool, timeout time.Duration) (*pcap.Handle, error) {
		if name != "eth-test" {
			t.Fatalf("interface name = %q, want eth-test", name)
		}
		if snaplen != 1024 {
			t.Fatalf("snaplen = %d, want 1024", snaplen)
		}
		if !promisc {
			t.Fatal("promisc = false, want true")
		}
		gotTimeout = timeout
		return nil, sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("openARPHandle error = %v, want %v", err, sentinel)
	}
	if gotTimeout <= 0 {
		t.Fatalf("capture timeout = %s, want a finite positive timeout", gotTimeout)
	}
}

func TestReadARPReplyStopsAfterDeadlineOnSilentCapture(t *testing.T) {
	reader := &timeoutPacketReader{}
	start := time.Unix(1, 0)
	now := start
	nowFn := func() time.Time {
		current := now
		now = now.Add(50 * time.Millisecond)
		return current
	}

	_, err := readARPReply(reader, []byte{192, 0, 2, 1}, start.Add(125*time.Millisecond), nowFn)
	if !errors.Is(err, errARPReplyTimeout) {
		t.Fatalf("readARPReply error = %v, want %v", err, errARPReplyTimeout)
	}
	if reader.reads != 3 {
		t.Fatalf("ReadPacketData calls = %d, want 3 before the deadline", reader.reads)
	}
}
