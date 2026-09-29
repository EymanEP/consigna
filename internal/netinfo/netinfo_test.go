package netinfo

import (
	"net/netip"
	"testing"
)

func TestClassifyAndSort(t *testing.T) {
	addrs := []Address{
		classify("docker0", netip.MustParseAddr("172.17.0.1")),
		classify("tailscale0", netip.MustParseAddr("100.101.1.2")),
		classify("eth0", netip.MustParseAddr("10.0.0.5")),
		classify("wlan0", netip.MustParseAddr("192.168.1.47")),
		classify("br-1234", netip.MustParseAddr("172.18.0.1")),
	}
	Sort(addrs)
	want := []string{"wlan0", "eth0", "tailscale0", "br-1234", "docker0"}
	for i, a := range addrs {
		if a.Interface != want[i] {
			t.Fatalf("order[%d] = %s, want %s (all: %+v)", i, a.Interface, want[i], addrs)
		}
	}
	kinds := map[string]Kind{"wlan0": KindWiFi, "eth0": KindEthernet, "tailscale0": KindVPN, "docker0": KindVirtual}
	for _, a := range addrs {
		if k, ok := kinds[a.Interface]; ok && a.Kind != k {
			t.Errorf("%s kind = %s, want %s", a.Interface, a.Kind, k)
		}
	}
}

func TestMacWiFi(t *testing.T) {
	if a := classify("en0", netip.MustParseAddr("192.168.0.3")); a.Kind != KindWiFi {
		t.Fatalf("en0 kind = %s", a.Kind)
	}
	if a := classify("Wi-Fi", netip.MustParseAddr("192.168.0.3")); a.Kind != KindWiFi {
		t.Fatalf("Windows Wi-Fi kind = %s", a.Kind)
	}
}

func TestAddressesSkipsLoopback(t *testing.T) {
	addrs, err := Addresses()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range addrs {
		if a.IP.IsLoopback() || !a.IP.Is4() {
			t.Fatalf("unexpected address %v", a.IP)
		}
	}
}
