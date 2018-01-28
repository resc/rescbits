package main

import (
	"github.com/resc/rescbits/bitbot/bitonic"
	"github.com/resc/rescbits/bitbot/datastore"
	log "github.com/sirupsen/logrus"
	"time"
	"github.com/resc/rescbits/bitbot/bus"
)

type (
	PriceAlert struct {
		Type             datastore.SampleType
		TriggerId        int64
		TriggerPrice     int64
		TriggerTimestamp time.Time
		SamplePrice      int64
		SampleTimestamp  time.Time
	}

	NewPriceAlertTrigger struct {
		Type            datastore.SampleType
		TriggerPrice    int64
		UserID          string
		ChannelID       string
		ResetHysteresis int64
	}

	DeletePriceAlertTrigger struct {
		Id int64
	}

	ArmPriceAlertTrigger struct {
		Id int64
	}
)

func priceAlerter(ds datastore.DataStore, triggersTopic, samplesTopic, alertsTopic bus.Topic) {
	ts := triggersTopic.Subscribe("priceAlerter")
	ss := samplesTopic.Subscribe("priceAlerter")

	triggers := <-ts.Messages()
	samples := <-ss.Messages()

	for {
		select {
		case t, ok := triggers:
			if ok {
				switch cmd := t.(type) {
				case NewPriceAlertTrigger:
					if uow, err := ds.StartUow(); err != nil {
						if trigger, err := uow.SaveAlert(datastore.PriceAlertTrigger{
							Type:                 cmd.Type,
							LastTriggerTimestamp: time.Date(0, 0, 0, 0, 0, 0, 0, nil),
							Price:                cmd.TriggerPrice,
							UserID:               cmd.UserID,
							ChannelID:            cmd.ChannelID,
							ResetHysteresis:      cmd.ResetHysteresis,
							IsArmed:              true,
						}); err != nil {

						} else {

						}
					} else {

					}
				case DeletePriceAlertTrigger:
				case ArmPriceAlertTrigger:
				}
			} else {
				triggers = nil
				if samples == nil {
					return
				}
			}
		case s, ok := samples:
			if ok {

			} else {
				samples = nil
				if triggers == nil {
					return
				}
			}
		}
	}
}

func (a *priceAlerter) Run() {
	defer func() {
		close(a.triggeredAlerts)
		// TODO add recovery
	}()

	for {
		select {
		case trigger := <-a.updatedTriggers:
			a.updateTrigger(trigger)
		case sample := <-a.samples:
			a.processSample(sample)
		case <-a.shutdown:
			log.Debug("Alerter shutting down...")
			return
		}
	}
}

func (a *priceAlerter) processSample(sample datastore.PriceSample) {
	log.Debugf("Processing sample %.2f (%v)", float64(sample.Price)/1e5, sample.Type)
	switch sample.Type {
	case datastore.SampleTypeBuy:
		a.processBuyTriggers(sample)
	case datastore.SampleTypeSell:
		a.processSellTriggers(sample)
	}
}

func (a *priceAlerter) processBuyTriggers(sample datastore.PriceSample) {
	for i := range a.buyTriggers {
		if a.buyTriggers[i].IsArmed {
			tripLimit := a.buyTriggers[i].Price
			price := sample.Price
			if price <= tripLimit {
				a.buyTriggers[i].IsArmed = false
				a.buyTriggers[i].LastTriggerTimestamp = time.Now()
				a.buyTriggers[i].TriggerCount += 1
				log.Infof("Trigger %d (%s) tripped because price (%.2f) dipped below %.2f %s", a.buyTriggers[i].Id, a.buyTriggers[i].Type, float64(price)/1e5, float64(tripLimit)/1e5, bitonic.CurrencyEur)
				a.triggeredAlerts <- PriceAlert{
					Type:             sample.Type,
					SamplePrice:      sample.Price,
					SampleTimestamp:  sample.Timestamp,
					TriggerId:        a.buyTriggers[i].Id,
					TriggerPrice:     a.buyTriggers[i].Price,
					TriggerTimestamp: a.buyTriggers[i].LastTriggerTimestamp,
				}

			}
		} else {
			reArmLimit := a.buyTriggers[i].Price + a.buyTriggers[i].ResetHysteresis
			if sample.Price >= reArmLimit {
				a.buyTriggers[i].IsArmed = true
				log.Infof("Re-armed trigger %d (%v) because price rose above %.2f %s", a.buyTriggers[i].Id, a.buyTriggers[i].Type, float64(reArmLimit)/1e5, bitonic.CurrencyEur)
			}
		}
	}
}

func (a *priceAlerter) processSellTriggers(sample datastore.PriceSample) {
	for i := range a.sellTriggers {
		if a.sellTriggers[i].Type != sample.Type {
			continue
		}

		if a.sellTriggers[i].IsArmed {
			tripLimit := a.sellTriggers[i].Price
			price := sample.Price
			if price >= tripLimit {
				a.sellTriggers[i].IsArmed = false
				a.sellTriggers[i].LastTriggerTimestamp = time.Now()
				a.sellTriggers[i].TriggerCount += 1
				log.Infof("Trigger %d (%s) tripped because price (%.2f) rose above %.2f %s", a.sellTriggers[i].Id, a.sellTriggers[i].Type, float64(price)/1e5, float64(tripLimit)/1e5, bitonic.CurrencyEur)
				a.triggeredAlerts <- PriceAlert{
					Type:             sample.Type,
					SamplePrice:      sample.Price,
					SampleTimestamp:  sample.Timestamp,
					TriggerId:        a.sellTriggers[i].Id,
					TriggerPrice:     a.sellTriggers[i].Price,
					TriggerTimestamp: a.sellTriggers[i].LastTriggerTimestamp,
				}
			}
		} else {
			reArmLimit := a.sellTriggers[i].Price - a.sellTriggers[i].ResetHysteresis
			if sample.Price <= reArmLimit {
				a.sellTriggers[i].IsArmed = true
				log.Infof("Armed trigger %d (%v) because price dropped below %.2f %s", a.sellTriggers[i].Id, a.sellTriggers[i].Type, float64(reArmLimit)/1e5, bitonic.CurrencyEur)
			}
		}
	}
}
func (a *priceAlerter) updateTrigger(trigger datastore.PriceAlertTrigger) {
	log.Infof("Updating trigger %d (%v)", trigger.Id, trigger.Type)

	switch trigger.Type {
	case datastore.SampleTypeBuy:
		for i := range a.buyTriggers {
			if a.buyTriggers[i].Id == trigger.Id {
				a.buyTriggers[i] = trigger
				return
			}
		}
		a.buyTriggers = append(a.buyTriggers, trigger)
	case datastore.SampleTypeSell:
		for i := range a.sellTriggers {
			if a.sellTriggers[i].Id == trigger.Id {
				a.sellTriggers[i] = trigger
				return
			}
		}
		a.sellTriggers = append(a.sellTriggers, trigger)
	}
}
