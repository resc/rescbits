package plots

import (
	"github.com/resc/rescbits/bitbot/datastore"
	"os"
	"testing"
	"time"
)

func TestRenderPlot(t *testing.T) {
	samples := buildSampleSlice()
	img, err := Render(samples)
	if err != nil {
		t.Fatal(err)
	}

	f, err := os.Create("c:\\temp\\test.png")
	if err != nil {
		t.Fatal(err)
	}

	defer f.Close()
	f.Write(img)
}

func buildSampleSlice() []datastore.PriceSample {
	start := time.Now().Add(-time.Hour)
	samples := make([]datastore.PriceSample, 100)
	for i := 0; i < len(samples)-1; i += 2 {
		t := start.Add(time.Duration(i) * time.Minute)
		samples[i] = datastore.PriceSample{
			Timestamp: t,
			Type:      datastore.SampleTypeBuy,
			Price:     (11000 + 10*int64(i)) * 1e5,
		}
		samples[i+1] = datastore.PriceSample{
			Timestamp: t,
			Type:      datastore.SampleTypeSell,
			Price:     (10000 + 10*int64(i)) * 1e5,
		}
	}
	return samples
}
