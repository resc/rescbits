package plots

import (
	"github.com/resc/rescbits/bitbot/datastore"
	"gonum.org/v1/plot/plotter"
	"math"
	"time"
)

type (
	sampleRange struct {
		samples   []datastore.PriceSample
		startDate time.Time
		endDate   time.Time
		minValue  int64
		maxValue  int64
	}
)

var _ plotter.XYer = (*sampleRange)(nil)

func newSampleRange(samples []datastore.PriceSample, sampleType datastore.SampleType) *sampleRange {
	// init start and end and min and max
	sr := &sampleRange{
		startDate: time.Date(9999, 1, 1, 0, 0, 0, 0, nil),
		endDate:   time.Date(0, 1, 1, 0, 0, 0, 0, nil),
		minValue:  math.MaxInt64,
		maxValue:  math.MinInt64,
	}

	sr.samples = make([]datastore.PriceSample, 0, len(samples))

	for _, s := range samples {
		if sampleType == datastore.SampleTypeNone || s.Type != sampleType {
			continue
		}

		if s.Timestamp.Before(sr.startDate) {
			sr.startDate = s.Timestamp
		}

		if s.Timestamp.After(sr.endDate) {
			sr.endDate = s.Timestamp
		}

		if s.Price < sr.minValue {
			sr.minValue = s.Price
		}

		if s.Price > sr.maxValue {
			sr.maxValue = s.Price
		}
		sr.samples = append(sr.samples, s)
	}

	return sr
}

func (sr *sampleRange) Len() int {
	return len(sr.samples)
}

func (sr *sampleRange) XY(i int) (x, y float64) {
	date := float64(sr.samples[i].Timestamp.Unix())
	price := float64(sr.samples[i].Price) / 1e5
	return date, price
}
