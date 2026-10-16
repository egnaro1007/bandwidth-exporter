package main

import (
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
	partnerID := flag.String("partner-id", envOr("PARTNER_ID", "3sg"), "partner label used for metrics")
	flag.Parse()

	collectors, err := loadCollectors(*pluginDir)
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
		var all []collector.Circuit
		for _, item := range collectors {
			values, err := item.c.Collect(r.Context())
			if err != nil {
				err = fmt.Errorf("plugin %s: %w", item.name, err)
				log.Print(err)
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			all = append(all, values...)
		}
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		if err := metrics.Render(w, all, *partnerID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	addr := ":" + *port
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}

func loadCollectors(dir string) ([]loadedCollector, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.so"))
	if err != nil {
		return nil, fmt.Errorf("scan plugins: %w", err)
	}
	result := make([]loadedCollector, 0, len(files))
	for _, file := range files {
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
		result = append(result, loadedCollector{name: filepath.Base(file), c: c})
	}
	return result, nil
}

func envOr(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}
