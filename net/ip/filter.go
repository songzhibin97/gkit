package ip

import (
	"net"
	"sync"
)

type Options struct {
	//AllowedIPs allowed IPs
	AllowedIPs []string
	//BlockedIPs blocked IPs
	BlockedIPs []string

	//block by default (defaults to allow)
	BlockByDefault bool
}

type subnet struct {
	str     string
	ipNet   *net.IPNet
	allowed bool
}

type Filter struct {
	opts Options
	//mut protects the below
	//rw since writes are rare
	mut            sync.RWMutex
	defaultAllowed bool
	ips            map[string]bool
	codes          map[string]bool
	subnets        []*subnet
}

func (f *Filter) AllowIP(ip string) bool {
	return f.ToggleIP(ip, true)
}

func (f *Filter) BlockIP(ip string) bool {
	return f.ToggleIP(ip, false)
}

func (f *Filter) ToggleIP(str string, allowed bool) bool {
	//check if has subnet
	if ip, net, err := net.ParseCIDR(str); err == nil {
		// containing only one ip? (no bits masked)
		if n, total := net.Mask.Size(); n == total {
			f.mut.Lock()
			f.ips[ip.String()] = allowed
			f.mut.Unlock()
			return true
		}
		//check for existing
		f.mut.Lock()
		found := false
		for _, subnet := range f.subnets {
			if subnet.str == str {
				found = true
				subnet.allowed = allowed
				break
			}
		}
		if !found {
			f.subnets = append(f.subnets, &subnet{
				str:     str,
				ipNet:   net,
				allowed: allowed,
			})
		}
		f.mut.Unlock()
		return true
	}
	//check if plain ip (/32)
	if ip := net.ParseIP(str); ip != nil {
		f.mut.Lock()
		f.ips[ip.String()] = allowed
		f.mut.Unlock()
		return true
	}
	return false
}

// ToggleDefault alters the default setting
func (f *Filter) ToggleDefault(allowed bool) {
	f.mut.Lock()
	f.defaultAllowed = allowed
	f.mut.Unlock()
}

// Allowed returns if a given IP can pass through the filter
func (f *Filter) Allowed(ipstr string) bool {
	return f.NetAllowed(net.ParseIP(ipstr))
}

// NetAllowed returns if a given net.IP can pass through the filter
func (f *Filter) NetAllowed(ip net.IP) bool {
	//invalid ip
	if ip == nil {
		return false
	}
	//read lock entire function
	//except for db access
	f.mut.RLock()
	defer f.mut.RUnlock()
	//check single ips
	allowed, ok := f.ips[ip.String()]
	if ok {
		return allowed
	}
	//scan subnets for any allow/block
	blocked := false
	for _, subnet := range f.subnets {
		if subnet.ipNet.Contains(ip) {
			if subnet.allowed {
				return true
			}
			blocked = true
		}
	}
	if blocked {
		return false
	}

	return f.defaultAllowed
}

// Blocked returns if a given IP can NOT pass through the filter
func (f *Filter) Blocked(ip string) bool {
	return !f.Allowed(ip)
}

// NetBlocked returns if a given net.IP can NOT pass through the filter
func (f *Filter) NetBlocked(ip net.IP) bool {
	return !f.NetAllowed(ip)
}

// New constructs IPFilter instance without downloading DB.
func New(opts Options) *Filter {
	f := &Filter{
		opts:           opts,
		ips:            map[string]bool{},
		codes:          map[string]bool{},
		defaultAllowed: !opts.BlockByDefault,
	}
	for _, ip := range opts.BlockedIPs {
		f.BlockIP(ip)
	}
	for _, ip := range opts.AllowedIPs {
		f.AllowIP(ip)
	}
	return f
}
