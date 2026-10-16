package metrics

import (
	"bytes"
	"strings"
	"testing"

	"bandwidth-exporter/internal/collector"
)

func TestRender(t *testing.T) {
	var output bytes.Buffer
	speed := 1000000000.0
	status := 1.0
	packetLoss := 0.0
	bandwidthIn := 42.0
	err := Render(&output, []collector.Circuit{{
		PartnerCircuitID: "circuit\"1",
		IfName:           "eth0",
		IfSpeed:          &speed,
		Status:           &status,
		PacketLoss:       &packetLoss,
		BandwidthInBPS:   &bandwidthIn,
	}}, "3sg")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	text := output.String()
	for _, expected := range []string{
		`circuit_info{if_name="eth0",if_speed="1000000000",partner_circuit_id="circuit\"1",partner_id="3sg"} 1`,
		`circuit_status{partner_circuit_id="circuit\"1",partner_id="3sg"} 1`,
		`circuit_packet_loss_percentage{partner_circuit_id="circuit\"1",partner_id="3sg"} 0`,
		`circuit_bandwidth_value_bps{partner_circuit_id="circuit\"1",partner_id="3sg",direction="in"} 42`,
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("output does not contain %q:\n%s", expected, text)
		}
	}
}

func TestRenderOmitsNullMetricsButKeepsRealZero(t *testing.T) {
	var output bytes.Buffer
	zero := 0.0
	err := Render(&output, []collector.Circuit{
		{PartnerCircuitID: "null-circuit"},
		{PartnerCircuitID: "zero-circuit", Status: &zero},
	}, "3sg")
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	text := output.String()
	if strings.Contains(text, `circuit_status{partner_circuit_id="null-circuit"`) {
		t.Fatal("null status must not be rendered")
	}
	if !strings.Contains(text, `circuit_status{partner_circuit_id="zero-circuit",partner_id="3sg"} 0`) {
		t.Fatal("real zero status must be rendered")
	}
	if strings.Contains(text, `circuit_bandwidth_value_bps{partner_circuit_id="null-circuit"`) {
		t.Fatal("null bandwidth must not be rendered")
	}
}
