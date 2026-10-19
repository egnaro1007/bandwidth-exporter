package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"plugin"
	"strings"

	"bandwidth-exporter/internal/collector"
	"bandwidth-exporter/internal/metrics"
)

type loadedCollector struct {
	name string
	c    collector.Collector
}

func main() {
	port := flag.String("port", envOr("PORT", "9598"), "HTTP listen port")
	pluginDir := flag.String("plugins", envOr("PLUGIN_DIR", "./plugins"), "directory containing .so plugins")
	flag.Parse()

	enabled := map[string]bool{
		"3sg":     envBool("ENABLE_3SG", true),
		"megamax": envBool("ENABLE_MEGAMAX", true),
	}
	collectors, err := loadCollectors(*pluginDir, enabled)
	if err != nil {
		log.Fatal(err)
	}
	if len(collectors) == 0 {
		log.Printf("no plugins loaded from %s", *pluginDir)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/circuits" {
			http.NotFound(w, r)
			return
		}
		var rendered bytes.Buffer
		for _, item := range collectors {
			values, err := item.c.Collect(r.Context())
			if err != nil {
				err = fmt.Errorf("plugin %s: %w", item.name, err)
				log.Print(err)
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			if err := metrics.Render(&rendered, values, item.c.PartnerID()); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write(rendered.Bytes())
	})

	addr := ":" + *port
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func loadCollectors(dir string, enabled map[string]bool) ([]loadedCollector, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.so"))
	if err != nil {
		return nil, fmt.Errorf("scan plugins: %w", err)
	}
	result := make([]loadedCollector, 0, len(files))
	found := make(map[string]bool)
	for _, file := range files {
		base := filepath.Base(file)
		if (base == "3sg.so" && !enabled["3sg"]) || (base == "megamax.so" && !enabled["megamax"]) {
			continue
		}
		p, err := plugin.Open(file)
		if err != nil {
			return nil, fmt.Errorf("load plugin %s: %w", file, err)
		}
		symbol, err := p.Lookup("NewCollector")
		if err != nil {
			return nil, fmt.Errorf("plugin %s does not export NewCollector: %w", file, err)
		}
		factory, ok := symbol.(func() collector.Collector)
		if !ok {
			return nil, fmt.Errorf("plugin %s NewCollector has type %T, want func() collector.Collector", file, symbol)
		}
		c := factory()
		if c == nil {
			return nil, fmt.Errorf("plugin %s returned a nil collector", file)
		}
		if configured, ok := enabled[c.PartnerID()]; ok && !configured {
			continue
		}
		found[c.PartnerID()] = true
		result = append(result, loadedCollector{name: base, c: c})
	}
	for partner, configured := range enabled {
		if configured && !found[partner] {
			return nil, fmt.Errorf("enabled partner %q plugin is not present in %s", partner, dir)
		}
	}
	return result, nil
}

func envBool(name string, fallback bool) bool {
	value, exists := os.LookupEnv(name)
	if !exists || strings.TrimSpace(value) == "" {
		return fallback
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	case "0", "false", "no", "n", "off":
		return false
	default:
		log.Printf("invalid %s=%q, using %t", name, value, fallback)
		return fallback
	}
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
