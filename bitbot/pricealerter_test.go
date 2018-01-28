package main

import (
	"testing"
	"github.com/resc/rescbits/bitbot/datastore"
	"time"
	log "github.com/sirupsen/logrus"
)

func TestPriceAlerter_Run(t *testing.T) {
	shutdown := make(chan struct{})
	updatedTriggers := make(chan datastore.PriceAlertTrigger)
	samples := make(chan datastore.PriceSample)

	triggers := RunNewPriceAlerter(updatedTriggers, samples, shutdown)
	go func() {
		log.Infof("closing shutdown channel in 5...")
		<-time.After(5 * time.Second)
		log.Infof("closing shutdown channel...")
		close(shutdown)
	}()
	go func() {
		for {
			select {
			case tg, ok := <-triggers:
				if !ok {
					log.Infof("triggers are done")
					return;
				} else {
					log.Infof("Price alert! trigger: %d %s, trigger price: %.2f sample price %.2f", tg.TriggerId, tg.Type, float64(tg.TriggerPrice)/1e5, float64(tg.SamplePrice)/1e5)
				}
			case <-time.After(time.Second):
				log.Infof("Tick...")
			}
		}
	}()

	updatedTriggers <- datastore.PriceAlertTrigger{
		Type:            datastore.SampleTypeBuy,
		Price:           100 * 1e5,
		TriggerCount:    0,
		IsArmed:         true,
		ResetHysteresis: 10 * 1e5,
		Id:              1,
		ChannelID:       "D123455",
		UserID:          "Kees",
	}

	marketPrices := []int64{102, 99, 100, 105, 98, 105, 110, 105, 98, 96, 95, 100, 103}
	for i := range marketPrices {
		samples <- datastore.PriceSample{
			Price:     marketPrices[i] * 1e5,
			Type:      datastore.SampleTypeBuy,
			Timestamp: time.Now(),
		}
		<-time.After(300 * time.Millisecond)
	}

	<-shutdown
	<-time.After(time.Second)

}
