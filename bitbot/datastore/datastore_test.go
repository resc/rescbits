package datastore

import (
	"github.com/pkg/errors"
	"testing"
	"time"
)

func TestUow_SavePriceSamples(t *testing.T) {
	ds, err := Open(TestDbConnStr)
	if err != nil {
		t.Fatal(errors.Wrap(err, "Error opening data store"))
	}
	defer ds.Close()
	err = ds.Ping()
	if err != nil {
		t.Fatal(errors.Wrap(err, "Error pinging data store"))
	}
	// check if pricesamples table is empty
	uow, err := ds.StartUow()
	if err != nil {
		t.Fatal(err)
	}

	samples, totalResults, err := uow.LoadPriceSamples(time.Now().Add(-60*60*24*7*52*time.Second), time.Now(), 100)
	if err != nil {
		t.Fatal(err)
	}
	if totalResults > 0 || len(samples) > 0 {
		t.Logf("Dirty test env, please clear pricessamples table")
	}

	err = uow.Commit()
	if err != nil {
		t.Fatal(err)
	}
	// check if we can insert some samples
	uow, err = ds.StartUow()
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()

	samples = make([]PriceSample, 10)
	for i := 0; i < len(samples); i++ {
		samples[i] = PriceSample{
			Price:     int64(i) * 1000,
			Timestamp: now.Add(time.Duration(-i) * time.Second),
			Type:      SampleTypeBuy,
		}
	}
	maxTime := samples[0].Timestamp.Add(1 * time.Second)
	minTime := samples[len(samples)-1].Timestamp.Add(-1 * time.Second)

	err = uow.SavePriceSamples(samples...)
	if err != nil {
		t.Fatal(err)
	}

	err = uow.Commit()
	if err != nil {
		t.Fatal(err)
	}

	// check if we can get the pricesamples out again
	uow, err = ds.StartUow()
	if err != nil {
		t.Fatal(err)
	}

	loadedSamples, loadedTotalResults, loadedErr := uow.LoadPriceSamples(minTime, maxTime, 100)
	if loadedErr != nil {
		t.Fatal(loadedErr)
	}

	if loadedTotalResults == 0 || len(loadedSamples) == 0 {
		t.Logf("Expected more pricessamples")
	}

	for i := range loadedSamples {
		t.Log(loadedSamples[i])
	}

	err = uow.Commit()
	if err != nil {
		t.Fatal(err)
	}

}
