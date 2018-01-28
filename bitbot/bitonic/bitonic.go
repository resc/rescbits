package bitonic

import (
	"time"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"io"
	"fmt"
	"strings"
	log "github.com/sirupsen/logrus"
	"github.com/resc/rescbits/bitbot/datastore"
)

const (
	ActionBuy   = "buy"
	ActionSell  = "sell"
	CurrencyEur = "eur"
	CurrencyBtc = "btc"
)

type (
	Api struct {
		buyUrl  string
		sellUrl string
	}

	PriceRequest struct {
		Action   string
		Currency string
		Amount   float64
	}

	PriceResponse struct {
		Request PriceRequest
		Time    time.Time
		Btc     float64
		Eur     float64
		Price   float64
		Error   string
	}
)

func New(buyUrl, sellUrl string) *Api {
	return &Api{
		buyUrl:  buyUrl,
		sellUrl: sellUrl,
	}
}

func (b *Api) RequestPrice(request *PriceRequest) <-chan *PriceResponse {
	response := make(chan *PriceResponse)
	go func() {
		defer func() {
			err := recover()
			if err != nil {
				errmsg := fmt.Sprint(err)
				if len(errmsg) == 0 {
					errmsg = "unknown error"
				}

				response <- &PriceResponse{
					Time:    time.Now(),
					Request: *request,
					Error:   errmsg,
				}
			}
			close(response)
		}()

		url, err := b.buildRequestUrl(request)
		panicIf(err)

		log.Printf("Sending request %s", url)
		resp, err := http.Get(url)
		panicIf(err)

		defer resp.Body.Close()
		bytes, err := ioutil.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		panicIf(err)

		log.Printf("Got response %s\n%s", resp.Status, string(bytes))
		priceData := &struct {
			Success bool    `json:"success"`
			Btc     float64 `json:"btc"`
			Eur     float64 `json:"eur"`
			Price   float64 `json:"price"`
			Method  string  `json:"method"`
			Error   string  `json:"error"`
		}{}

		err = json.Unmarshal(bytes, priceData)
		panicIf(err)

		if !priceData.Success || priceData.Error != "" {
			response <- &PriceResponse{
				Time:    time.Now(),
				Request: *request,
				Error:   fmt.Sprintf("Bitonic said: %s", priceData.Error),
			}
		} else {
			response <- &PriceResponse{
				Time:    time.Now(),
				Request: *request,
				Btc:     priceData.Btc,
				Eur:     priceData.Eur,
				Price:   priceData.Price,
			}
		}
	}()

	return response
}

func (b *Api) buildRequestUrl(request *PriceRequest) (string, error) {
	action := strings.ToLower(request.Action)
	currency := strings.ToLower(request.Currency)
	url := ""

	switch action {
	case ActionBuy:
		url = b.buyUrl
	case ActionSell:
		url = b.sellUrl
	default:
		return "", fmt.Errorf("invalid action: %s", request.Action)
	}

	switch currency {
	case CurrencyBtc:
		url = fmt.Sprintf("%s?%s=%f", url, CurrencyBtc, request.Amount)
	case CurrencyEur:
		url = fmt.Sprintf("%s?%s=%f", url, CurrencyEur, request.Amount)
	default:
		return "", fmt.Errorf("invalid currency: %s", request.Currency)
	}

	return url, nil

}

func PricePoller(bitonicApi *Api, ds datastore.DataStore, pollInterval time.Duration, shutdown <-chan struct{}) {
	defer func() {

		err := recover()
		if err != nil {
			log.Errorf("bitonicpPricePoller: %v", err)
			duration := 60 * time.Second
			log.Infof("bitonicpPricePoller: suspended due to error, resuming in %v", duration)
			select {
			case <-shutdown:
				return // quick exit on shutdown
			case <-time.After(duration):
				log.Infof("bitonicpPricePoller: resumed polling")
				go PricePoller(bitonicApi, ds, pollInterval, shutdown)
			}
		}
	}()
	for {
		select {
		case <-shutdown:
			return
		case <-time.After(pollInterval):
			buyResponse := bitonicApi.RequestPrice(&PriceRequest{
				Amount:   1,
				Currency: CurrencyBtc,
				Action:   ActionBuy,
			})
			sellResponse := bitonicApi.RequestPrice(&PriceRequest{
				Amount:   1,
				Currency: CurrencyBtc,
				Action:   ActionSell,
			})
			if uow, err := ds.StartUow(); err != nil {
				log.Errorf("")
			} else {
				func() {
					defer uow.Commit()

					buy := <-buyResponse
					if buy.Error == "" {
						err := uow.SavePriceSamples(datastore.PriceSample{
							Type:      "B",
							Timestamp: buy.Time,
							Price:     int64(buy.Price * 1e5),
						})
						if err != nil {
							log.Errorf("Error saving price sample", err)
						}
					} else {
						log.Errorf("Error fetching buy price: %s", buy.Error)
					}

					sell := <-sellResponse
					if sell.Error == "" {
						err := uow.SavePriceSamples(datastore.PriceSample{
							Type:      "S",
							Timestamp: sell.Time,
							Price:     int64(sell.Price * 1e5),
						})
						if err != nil {
							log.Errorf("Error saving price sample", err)
						}
					} else {
						log.Errorf("Error fetching buy price: %s", sell.Error)
					}
				}()
			}
		}
	}
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
