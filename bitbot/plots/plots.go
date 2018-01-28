package plots

import (
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"

	"bytes"
	"fmt"
	"github.com/resc/rescbits/bitbot/datastore"
	"math"
	_ "math"
	"time"
)

// Render renders the image as a png and returns the image as a byte slice
func Render(samples []datastore.PriceSample) ([]byte, error) {
	p, err := plot.New()
	if err != nil {
		panic(err)
	}

	buys := newSampleRange(samples, datastore.SampleTypeBuy)
	sells := newSampleRange(samples, datastore.SampleTypeSell)

	xTicks := computeTicks(samples)

	p.Title.Text = "example"
	p.X.Label.Text = "Time"
	p.X.Min = xTicks[0].Value
	p.X.Max = xTicks[len(xTicks)-1].Value
	p.X.Tick.Marker = xTicks
	p.X.Tick.Label.Rotation = d2r(30)
	p.X.Tick.Label.XAlign = draw.XRight
	p.X.Tick.Label.YAlign = draw.YTop

	p.Y.Label.Text = "Price"
	p.Y.Tick.Marker = euroTickMarker{}

	err = plotutil.AddLinePoints(p, "Buy", buys, "Sell", sells)
	if err != nil {
		return nil, err
	}

	// Save the plot to a buffer.
	if w, err := p.WriterTo(12*vg.Inch, 4*vg.Inch, "png"); err != nil {
		return nil, err
	} else {
		img := new(bytes.Buffer)
		_, err := w.WriteTo(img)
		if err != nil {
			return nil, err
		}
		return img.Bytes(), nil
	}
}

func computeTicks(samples []datastore.PriceSample) plot.ConstantTicks {
	sr := newSampleRange(samples, "")
	sd := roundDownToMinute(sr.startDate)
	ed := roundDownToMinute(sr.endDate).Add(2 * time.Minute)
	inc, tickLabelInterval := getIncrementAndLabelInterval(ed.Sub(sd))

	xTicks := plot.ConstantTicks{}
	for sd.Before(ed) {
		label := ""
		if sd.Minute()%tickLabelInterval == 0 {
			label = sd.Format(time.Kitchen)
		}
		xTicks = append(xTicks, plot.Tick{
			Value: float64(sd.Unix()),
			Label: label,
		})
		sd = sd.Add(inc)
	}
	return xTicks
}

func getIncrementAndLabelInterval(d time.Duration) (time.Duration, int) {
	interval := 0
	inc := time.Second
	if d.Minutes() < 30 {
		interval = 5
		inc = time.Minute
	} else if d.Minutes() < 60 {
		interval = 10
		inc = 2 * time.Minute
	} else if d.Minutes() < 120 {
		interval = 15
		inc = 5 * time.Minute
	} else if d.Minutes() < 240 {
		interval = 30
		inc = 10 * time.Minute
	} else if d.Minutes() < 480 {
		interval = 60
		inc = 15 * time.Minute
	} else if d.Minutes() < 960 {
		interval = 120
		inc = 30 * time.Minute
	} else {
		interval = 240
		inc = 60 * time.Minute
	}
	return inc, interval
}

func roundDownToMinute(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
}

func d2r(degrees float64) float64 {
	radians := degrees * math.Pi / 180
	return radians
}

type euroTickMarker struct{}

var _ plot.Ticker = euroTickMarker{}

func (euroTickMarker) Ticks(min, max float64) []plot.Tick {
	//bottom := math.Floor(min/1000) * 1000
	//top := math.Ceil(max/1000) * 1000
	tks := plot.DefaultTicks{}.Ticks(min, max)
	for i, t := range tks {
		if t.Label == "" { // Skip minor ticks, they are fine.
			continue
		}
		tks[i].Label = fmt.Sprintf("%.2f €", tks[i].Value)
	}
	return tks
}
