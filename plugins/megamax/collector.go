package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"bandwidth-exporter/internal/collector"
)

type megamaxCollector struct {
	url      string
	apiToken string
	client   *http.Client
}

func (m *megamaxCollector) PartnerID() string {
	return "megamax"
}

func NewCollector() collector.Collector {
	insecure := envBool("MEGAMAX_INSECURE_SKIP_VERIFY")
	if insecure {
		log.Print("WARNING: MEGAMAX_INSECURE_SKIP_VERIFY is enabled; TLS certificates are not verified")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: insecure} //nolint:gosec // explicit opt-in via environment
	return &megamaxCollector{
		url:      os.Getenv("MEGAMAX_URL"),
		apiToken: os.Getenv("MEGAMAX_API_TOKEN"),
		client:   &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

type portsResponse map[string]statusGroup

type statusGroup struct {
	Ports map[string]port `json:"ports"`
}

type port struct {
	Description    interface{}          `json:"description"`
	PortID         interface{}          `json:"port_id"`
	IfName         interface{}          `json:"ifName"`
	IfSpeed        interface{}          `json:"ifSpeed"`
	BandwidthGraph map[string]bandwidth `json:"bandwidth_graphs"`
}

type bandwidth struct {
	InBPS  interface{} `json:"in_bps"`
	OutBPS interface{} `json:"out_bps"`
}

func (m *megamaxCollector) Collect(ctx context.Context) ([]collector.Circuit, error) {
	if strings.TrimSpace(m.url) == "" {
		return nil, fmt.Errorf("MEGAMAX_URL is required")
	}
	if strings.TrimSpace(m.apiToken) == "" {
		return nil, fmt.Errorf("MEGAMAX_API_TOKEN is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return nil, fmt.Errorf("create Megamax request: %w", err)
	}
	req.Header.Set("X-API-Token", m.apiToken)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Megamax API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("Megamax API returned HTTP %s", resp.Status)
	}
	var data portsResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("decode Megamax response: %w", err)
	}

	var result []collector.Circuit
	for groupName, group := range data {
		status, ok := megamaxStatus(groupName)
		if !ok {
			continue
		}
		for fallbackID, p := range group.Ports {
			result = append(result, circuitFromPort(p, fallbackID, status))
		}
	}
	return result, nil
}

func megamaxStatus(group string) (*float64, bool) {
	switch strings.ToLower(strings.TrimSpace(group)) {
	case "ok", "status: ok":
		return floatPointer(0), true
	case "fail", "status: fail":
		return floatPointer(1), true
	default:
		return nil, false
	}
}

func circuitFromPort(p port, fallbackID string, status *float64) collector.Circuit {
	circuitID := stringValue(p.Description)
	if circuitID == "" {
		circuitID = fallbackID
	}
	c := collector.Circuit{
		PartnerCircuitID: circuitID,
		IfName:           stringValue(p.IfName),
		IfSpeed:          numberValue(p.IfSpeed),
		Status:           status,
	}
	if normal, ok := p.BandwidthGraph["6_hours"]; ok {
		c.BandwidthInBPS = numberValue(normal.InBPS)
		c.BandwidthOutBPS = numberValue(normal.OutBPS)
	}
	return c
}

func stringValue(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func numberValue(value interface{}) *float64 {
	if value == nil {
		return nil
	}
	var parsed float64
	switch v := value.(type) {
	case float64:
		parsed = v
	case json.Number:
		var err error
		parsed, err = v.Float64()
		if err != nil {
			return nil
		}
	case string:
		var err error
		parsed, err = strconv.ParseFloat(strings.TrimSpace(v), 64)
		if err != nil {
			return nil
		}
	default:
		return nil
	}
	return &parsed
}

func envBool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func floatPointer(value float64) *float64 {
	return &value
}
