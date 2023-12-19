package arp

import (
	"errors"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/routing"
)

type ExtraInfo struct {
	IFace     *net.Interface
	DstHWAddr net.HardwareAddr
	SrcIP     net.IP
}

func GetRouterInfo(dstIp net.IP) (*ExtraInfo, error) {
	router, err := routing.New()
	if err != nil {
		return nil, err
	}

	extraInfo := &ExtraInfo{}
	iFace, gateway, preferredSrc, err := router.Route(dstIp)
	if err != nil {
		return nil, err
	}
	extraInfo.IFace = iFace
	extraInfo.SrcIP = preferredSrc
	handle, err := pcap.OpenLive(iFace.Name, 1024, true, pcap.BlockForever)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	dstHWAddr, err := getHWAddr(dstIp, gateway, preferredSrc, iFace, handle)
	if err != nil {
		return nil, err
	}
	extraInfo.DstHWAddr = dstHWAddr
	return extraInfo, nil
}

func getHWAddr(ip, gateway, srcIP net.IP, networkInterface *net.Interface, handle *pcap.Handle) (net.HardwareAddr, error) {
	arpDst := ip
	if gateway != nil {
		arpDst = gateway
	}

	eth := layers.Ethernet{
		SrcMAC:       networkInterface.HardwareAddr,
		DstMAC:       net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff},
		EthernetType: layers.EthernetTypeARP,
	}

	arp := layers.ARP{
		AddrType:          layers.LinkTypeEthernet,
		Protocol:          layers.EthernetTypeIPv4,
		HwAddressSize:     uint8(6),
		ProtAddressSize:   uint8(4),
		Operation:         layers.ARPRequest,
		SourceHwAddress:   []byte(networkInterface.HardwareAddr),
		SourceProtAddress: srcIP,
		DstHwAddress:      net.HardwareAddr{0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
		DstProtAddress:    arpDst,
	}

	opt := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}

	buf := gopacket.NewSerializeBuffer()

	if err := gopacket.SerializeLayers(buf, opt, &eth, &arp); err != nil {
		return nil, err
	}
	if err := handle.WritePacketData(buf.Bytes()); err != nil {
		return nil, err
	}

	start := time.Now()
	for {
		if time.Since(start) > time.Millisecond*time.Duration(1000) {
			return nil, errors.New("timeout getting ARP reply")
		}
		data, _, err := handle.ReadPacketData()
		if errors.Is(err, pcap.NextErrorTimeoutExpired) {
			continue
		} else if err != nil {
			return nil, err
		}
		packet := gopacket.NewPacket(data, layers.LayerTypeEthernet, gopacket.NoCopy)
		if arpLayer := packet.Layer(layers.LayerTypeARP); arpLayer != nil {
			arp := arpLayer.(*layers.ARP)
			if net.IP(arp.SourceProtAddress).Equal(arpDst) {
				return net.HardwareAddr(arp.SourceHwAddress), nil
			}
		}
	}
}
