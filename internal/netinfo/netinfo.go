// Package netinfo finds the addresses other devices can use to reach this
// machine, best candidate first.
package netinfo

import (
	"net"
	"net/netip"
	"slices"
	"strings"
)

// Kind is a best guess at what an interface is.
type Kind string

// Interface kinds.
const (
	KindWiFi     Kind = "wifi"
	KindEthernet Kind = "ethernet"
	KindVPN      Kind = "vpn"
	KindVirtual  Kind = "virtual"
	KindOther    Kind = "other"
)

// Address is one way to reach this machine.
type Address struct {
	Interface string
	IP        netip.Addr
	Kind      Kind
	score     int
}

// Addresses lists usable IPv4 addresses on interfaces that are up, the most
// likely LAN address first. Loopback and link-local addresses are skipped.
func Addresses() ([]Address, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	var out []Address
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifc.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			prefix, err := netip.ParsePrefix(a.String())
			if err != nil {
				continue
			}
			ip := prefix.Addr().Unmap()
			if !ip.Is4() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
				continue
			}
			out = append(out, classify(ifc.Name, ip))
		}
	}
	Sort(out)
	return out, nil
}

// Sort orders addresses best first.
func Sort(addrs []Address) {
	slices.SortStableFunc(addrs, func(a, b Address) int {
		if a.score != b.score {
			return b.score - a.score
		}
		if c := strings.Compare(a.Interface, b.Interface); c != 0 {
			return c
		}
		return a.IP.Compare(b.IP)
	})
}

var virtualPrefixes = []string{
	"docker", "br-", "veth", "virbr", "vmnet", "vboxnet", "vethernet", "lxc",
	"lxd", "podman", "cni", "flannel", "kube", "awdl", "llw", "bridge", "ap1",
	"anpi", "gif", "stf", "ham",
}

var vpnPrefixes = []string{"tun", "tap", "wg", "utun", "tailscale", "zt", "ipsec", "ppp", "nordlynx", "proton"}

func classify(name string, ip netip.Addr) Address {
	n := strings.ToLower(name)
	a := Address{Interface: name, IP: ip, Kind: KindOther}
	switch {
	case hasAnyPrefix(n, virtualPrefixes) || strings.Contains(n, "virtual") || strings.Contains(n, "hyper-v"):
		a.Kind = KindVirtual
	case hasAnyPrefix(n, vpnPrefixes) || isCGNAT(ip):
		a.Kind = KindVPN
	case strings.HasPrefix(n, "wl") || strings.Contains(n, "wi-fi") || strings.Contains(n, "wifi") ||
		strings.Contains(n, "wireless") || n == "en0":
		// en0 is the built-in Wi-Fi on nearly every Mac laptop.
		a.Kind = KindWiFi
	case strings.HasPrefix(n, "eth") || strings.HasPrefix(n, "en") || strings.Contains(n, "ethernet"):
		a.Kind = KindEthernet
	}

	switch a.Kind {
	case KindWiFi:
		a.score = 50
	case KindEthernet:
		a.score = 45
	case KindOther:
		a.score = 30
	case KindVPN:
		a.score = 10
	case KindVirtual:
		a.score = 0
	}
	switch {
	case isPrefix(ip, "192.168.0.0/16"):
		a.score += 5
	case isPrefix(ip, "10.0.0.0/8"):
		a.score += 4
	case isPrefix(ip, "172.16.0.0/12"):
		a.score += 3
	}
	return a
}

func hasAnyPrefix(s string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func isCGNAT(ip netip.Addr) bool { return isPrefix(ip, "100.64.0.0/10") }

func isPrefix(ip netip.Addr, cidr string) bool {
	return netip.MustParsePrefix(cidr).Contains(ip)
}
