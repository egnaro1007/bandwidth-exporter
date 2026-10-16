package main

import "testing"

func TestEnvBool(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv("THREESG_ZABBIX_INSECURE_SKIP_VERIFY", value)
		if !envBool("THREESG_ZABBIX_INSECURE_SKIP_VERIFY") {
			t.Errorf("envBool(%q) = false, want true", value)
		}
	}
	for _, value := range []string{"", "false", "no", "off", "invalid"} {
		t.Setenv("THREESG_ZABBIX_INSECURE_SKIP_VERIFY", value)
		if envBool("THREESG_ZABBIX_INSECURE_SKIP_VERIFY") {
			t.Errorf("envBool(%q) = true, want false", value)
		}
	}
}

func TestCircuitFromHost(t *testing.T) {
	h := host{
		Macros: []struct {
			Macro string `json:"macro"`
			Value string `json:"value"`
		}{{Macro: "{$WAN1_ACC}", Value: "acc-1"}},
		Items: []struct {
			Name      string `json:"name"`
			Key       string `json:"key_"`
			LastValue string `json:"lastvalue"`
		}{
			{Name: "Interface wan1(eth0): speed", Key: "net.if.speed[eth0]", LastValue: "1000000000"},
			{Name: "Interface wan1(eth0): inbound", Key: "net.if.in[eth0]", LastValue: "12"},
			{Name: "Interface wan1(eth0): errors", Key: "net.if.in.errors[eth0]", LastValue: "3"},
			{Name: "ICMP", Key: "icmpping[{$WAN1_IP}]", LastValue: "1"},
		},
	}
	c := circuitFromHost(h, "wan1", macroValue(h, "{$WAN1_ACC}"))
	if c.PartnerCircuitID != "acc-1" || c.IfName != "eth0" || c.IfSpeed == nil || *c.IfSpeed != 1000000000 ||
		c.BandwidthInBPS == nil || *c.BandwidthInBPS != 12 || c.ErrorsInTotal == nil || *c.ErrorsInTotal != 3 ||
		c.Status == nil || *c.Status != 1 {
		t.Fatalf("unexpected circuit: %+v", c)
	}
}

func TestCircuitFromHostKeepsMissingItemsNil(t *testing.T) {
	c := circuitFromHost(host{}, "wan1", "acc-1")
	if c.IfSpeed != nil || c.Status != nil || c.LatencySeconds != nil ||
		c.BandwidthInBPS != nil || c.ErrorsInTotal != nil {
		t.Fatalf("missing items must remain nil: %+v", c)
	}
}
