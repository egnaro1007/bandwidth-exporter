package collector

import "context"

type Circuit struct {
	PartnerCircuitID string
	IfName           string
	IfSpeed          *float64
	Status           *float64
	LatencySeconds   *float64
	PacketLoss       *float64
	BandwidthInBPS   *float64
	BandwidthOutBPS  *float64
	ErrorsInTotal    *float64
	ErrorsOutTotal   *float64
	DiscardsInTotal  *float64
	DiscardsOutTotal *float64
}

type Collector interface {
	Collect(context.Context) ([]Circuit, error)
}
