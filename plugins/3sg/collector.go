package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"bandwidth-exporter/internal/collector"
)

type zabbixCollector struct {
	url       string
	authToken string
	client    *http.Client
	username  string
	password  string
}

func NewCollector() collector.Collector {
	return &zabbixCollector{
		url:       os.Getenv("THREESG_ZABBIX_URL"),
		authToken: os.Getenv("THREESG_ZABBIX_AUTH_TOKEN"),
		username:  os.Getenv("THREESG_ZABBIX_USERNAME"),
		password:  os.Getenv("THREESG_ZABBIX_PASSWORD"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

type rpcRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
	Auth    string      `json:"auth,omitempty"`
	ID      int         `json:"id"`
}

type host struct {
	Name   string `json:"name"`
	Macros []struct {
		Macro string `json:"macro"`
		Value string `json:"value"`
	} `json:"macros"`
	Items []struct {
		Name      string `json:"name"`
		Key       string `json:"key_"`
		LastValue string `json:"lastvalue"`
	} `json:"items"`
}

type rpcResponse struct {
	Result []host    `json:"result"`
	Error  *rpcError `json:"error"`
}

type rpcError struct {
	Message string `json:"message"`
	Data    string `json:"data"`
}

var interfaceName = regexp.MustCompile(`(?i)^Interface\s+\w+\((.*?)\)`)

func (z *zabbixCollector) Collect(ctx context.Context) ([]collector.Circuit, error) {
	if strings.TrimSpace(z.url) == "" {
		return nil, fmt.Errorf("THREESG_ZABBIX_URL is required")
	}
	authToken := z.authToken
	if authToken == "" && z.username != "" {
		var err error
		authToken, err = z.login(ctx)
		if err != nil {
			return nil, err
		}
	}
	params := map[string]interface{}{
		"output":       []string{"hostid", "name"},
		"search":       map[string]string{"name": "PJICO_"},
		"startSearch":  true,
		"selectMacros": []string{"macro", "value"},
		"selectItems":  []string{"itemid", "name", "key_", "lastvalue"},
	}
	var response rpcResponse
	if err := z.call(ctx, "host.get", params, authToken, &response); err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, fmt.Errorf("zabbix host.get: %s: %s", response.Error.Message, response.Error.Data)
	}
	var result []collector.Circuit
	for _, h := range response.Result {
		for _, wan := range []string{"wan1", "wan2"} {
			acc := macroValue(h, "{$"+strings.ToUpper(wan)+"_ACC}")
			if acc == "" {
				continue
			}
			result = append(result, circuitFromHost(h, wan, acc))
		}
	}
	return result, nil
}

func (z *zabbixCollector) login(ctx context.Context) (string, error) {
	params := map[string]string{"username": z.username, "password": z.password}
	var response struct {
		Result string    `json:"result"`
		Error  *rpcError `json:"error"`
	}
	if err := z.call(ctx, "user.login", params, "", &response); err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", fmt.Errorf("zabbix login: %s: %s", response.Error.Message, response.Error.Data)
	}
	if response.Result == "" {
		return "", fmt.Errorf("zabbix login returned an empty token")
	}
	return response.Result, nil
}

func (z *zabbixCollector) call(ctx context.Context, method string, params interface{}, auth string, output interface{}) error {
	request := rpcRequest{JSONRPC: "2.0", Method: method, Params: params, ID: 1}
	if method == "user.login" {
		request.Auth = auth
	}
	body, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode %s request: %w", method, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, z.url, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create %s request: %w", method, err)
	}
	req.Header.Set("Content-Type", "application/json-rpc")
	if method != "user.login" && auth != "" {
		req.Header.Set("Authorization", "Bearer "+auth)
	}
	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("call %s: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("call %s returned HTTP %s", method, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
		return fmt.Errorf("decode %s response: %w", method, err)
	}
	return nil
}

func macroValue(h host, name string) string {
	for _, macro := range h.Macros {
		if macro.Macro == name {
			return strings.TrimSpace(macro.Value)
		}
	}
	return ""
}

func circuitFromHost(h host, wan, acc string) collector.Circuit {
	c := collector.Circuit{PartnerCircuitID: acc}
	c.IfName = interfaceNameFor(h, wan)
	c.IfSpeed = itemNumber(h, wan, func(key string) bool { return strings.HasPrefix(key, "net.if.speed[") }, false)
	c.Status = itemNumber(h, "", func(key string) bool { return key == "icmpping[{$"+strings.ToUpper(wan)+"_IP}]" }, false)
	c.LatencySeconds = itemNumber(h, "", func(key string) bool { return key == "icmppingsec[{$"+strings.ToUpper(wan)+"_IP}]" }, false)
	c.PacketLoss = itemNumber(h, "", func(key string) bool { return key == "icmppingloss[{$"+strings.ToUpper(wan)+"_IP}]" }, false)
	c.BandwidthInBPS = itemNumber(h, wan, func(key string) bool { return strings.HasPrefix(key, "net.if.in[") }, false)
	c.BandwidthOutBPS = itemNumber(h, wan, func(key string) bool { return strings.HasPrefix(key, "net.if.out[") }, false)
	c.ErrorsInTotal = interfaceMetric(h, wan, "in", "errors")
	c.ErrorsOutTotal = interfaceMetric(h, wan, "out", "errors")
	c.DiscardsInTotal = interfaceMetric(h, wan, "in", "discards")
	c.DiscardsOutTotal = interfaceMetric(h, wan, "out", "discards")
	return c
}

func interfaceNameFor(h host, wan string) string {
	for _, item := range h.Items {
		if strings.HasPrefix(item.Name, "Interface "+wan+"(") {
			match := interfaceName.FindStringSubmatch(item.Name)
			if len(match) == 2 {
				return match[1]
			}
		}
	}
	return ""
}

func interfaceMetric(h host, wan, direction, metric string) *float64 {
	return itemNumber(h, wan, func(key string) bool {
		return strings.HasPrefix(key, "net.if."+direction+"."+metric+"[")
	}, true)
}

func itemNumber(h host, wan string, match func(string) bool, integer bool) *float64 {
	for _, item := range h.Items {
		if (wan == "" || strings.HasPrefix(item.Name, "Interface "+wan+"(")) && match(item.Key) {
			value, err := strconv.ParseFloat(strings.TrimSpace(item.LastValue), 64)
			if err == nil {
				if integer {
					value = float64(int64(value))
				}
				return &value
			}
		}
	}
	return nil
}
