package main

import (
	"context"
	"strings"
	"testing"
)

func TestMegamaxCollectorRequiresURL(t *testing.T) {
	err := (&megamaxCollector{}).collectError(context.Background())
	if err == nil || !strings.Contains(err.Error(), "MEGAMAX_URL is required") {
		t.Fatalf("error = %v, want missing URL error", err)
	}
}

func TestMegamaxCollectorRequiresToken(t *testing.T) {
	err := (&megamaxCollector{url: "http://example.invalid"}).collectError(context.Background())
	if err == nil || !strings.Contains(err.Error(), "MEGAMAX_API_TOKEN is required") {
		t.Fatalf("error = %v, want missing API token error", err)
	}
}

func (m *megamaxCollector) collectError(ctx context.Context) error {
	_, err := m.Collect(ctx)
	return err
}

func TestMegamaxStatus(t *testing.T) {
	tests := []struct {
		group string
		want  float64
		ok    bool
	}{
		{group: "ok", want: 0, ok: true},
		{group: "OK", want: 0, ok: true},
		{group: "status: ok", want: 0, ok: true},
		{group: "fail", want: 1, ok: true},
		{group: "status: fail", want: 1, ok: true},
		{group: "unknown", ok: false},
	}
	for _, tt := range tests {
		got, ok := megamaxStatus(tt.group)
		if ok != tt.ok {
			t.Fatalf("megamaxStatus(%q) ok = %v, want %v", tt.group, ok, tt.ok)
		}
		if ok && (got == nil || *got != tt.want) {
			t.Fatalf("megamaxStatus(%q) = %v, want %v", tt.group, got, tt.want)
		}
	}
}

func TestCircuitFromPortUsesOnlyNormalBandwidth(t *testing.T) {
	status := floatPointer(0)
	p := port{
		Description: "circuit-1",
		IfName:      "eth0",
		IfSpeed:     float64(100000000000),
		BandwidthGraph: map[string]bandwidth{
			"6_hours": {InBPS: float64(12), OutBPS: "34"},
			"1_hour":  {InBPS: float64(99), OutBPS: float64(99)},
		},
	}
	c := circuitFromPort(p, "fallback", status)
	if c.PartnerCircuitID != "circuit-1" || c.IfName != "eth0" ||
		c.IfSpeed == nil || *c.IfSpeed != 100000000000 ||
		c.BandwidthInBPS == nil || *c.BandwidthInBPS != 12 ||
		c.BandwidthOutBPS == nil || *c.BandwidthOutBPS != 34 ||
		c.LatencySeconds != nil || c.PacketLoss != nil ||
		c.ErrorsInTotal != nil || c.DiscardsInTotal != nil {
		t.Fatalf("unexpected circuit: %+v", c)
	}
}

func TestCircuitFromPortOmitsNullValues(t *testing.T) {
	c := circuitFromPort(port{
		Description: nil,
		BandwidthGraph: map[string]bandwidth{
			"6_hours": {InBPS: nil, OutBPS: nil},
		},
	}, "fallback", floatPointer(1))
	if c.PartnerCircuitID != "fallback" || c.IfSpeed != nil ||
		c.BandwidthInBPS != nil || c.BandwidthOutBPS != nil {
		t.Fatalf("null values must remain absent: %+v", c)
	}
}
