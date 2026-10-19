package metrics

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"bandwidth-exporter/internal/collector"
)

func Render(w io.Writer, circuits []collector.Circuit, partnerID string) error {
	labels := func(c collector.Circuit) string {
		return fmt.Sprintf(`partner_circuit_id="%s",partner_id="%s"`,
			escape(c.PartnerCircuitID), escape(partnerID))
	}
	infoLabels := func(c collector.Circuit) string {
		speed := ""
		if c.IfSpeed != nil {
			speed = formatInteger(*c.IfSpeed)
		}
		return fmt.Sprintf(`if_name="%s",if_speed="%s",partner_circuit_id="%s",partner_id="%s"`,
			escape(c.IfName), speed, escape(c.PartnerCircuitID), escape(partnerID))
	}
	write := func(name, label string, value float64) error {
		_, err := fmt.Fprintf(w, "%s{%s} %s\n", name, label, format(value))
		return err
	}

	for _, c := range circuits {
		if err := write("circuit_info", infoLabels(c), 1); err != nil {
			return err
		}
		for _, metric := range []struct {
			name  string
			value *float64
		}{
			{"circuit_status", c.Status},
			{"circuit_latency_seconds", c.LatencySeconds},
			{"circuit_packet_loss_percentage", c.PacketLoss},
			{"circuit_bandwidth_value_bps{" + labels(c) + `,direction="in"}`, c.BandwidthInBPS},
			{"circuit_bandwidth_value_bps{" + labels(c) + `,direction="out"}`, c.BandwidthOutBPS},
			{"circuit_packet_errors_total{" + labels(c) + `,direction="in"}`, c.ErrorsInTotal},
			{"circuit_packet_errors_total{" + labels(c) + `,direction="out"}`, c.ErrorsOutTotal},
			{"circuit_packet_discards_total{" + labels(c) + `,direction="in"}`, c.DiscardsInTotal},
			{"circuit_packet_discards_total{" + labels(c) + `,direction="out"}`, c.DiscardsOutTotal},
		} {
			if metric.value == nil {
				continue
			}
			if strings.Contains(metric.name, "{") {
				if _, err := fmt.Fprintf(w, "%s %s\n", metric.name, format(*metric.value)); err != nil {
					return err
				}
				continue
			}
			if err := write(metric.name, labels(c), *metric.value); err != nil {
				return err
			}
		}
	}
	return nil
}

func format(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func formatInteger(v float64) string {
	return strconv.FormatFloat(v, 'f', 0, 64)
}

func escape(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, `"`, `\"`)
	return strings.ReplaceAll(v, "\n", `\n`)
}
