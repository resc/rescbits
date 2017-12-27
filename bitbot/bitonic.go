package main

import (
	"time"
	"net/http"
	"encoding/json"
	"io/ioutil"
	"io"
	"fmt"
	"strings"
	"log"
)

const (
	ActionBuy   = "buy"
	ActionSell  = "sell"
	CurrencyEur = "eur"
	CurrencyBtc = "btc"
)

type (
	bitonicAgent struct {
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

func NewBitonicAgent(buyUrl, sellUrl string) *bitonicAgent {
	return &bitonicAgent{
		buyUrl:  buyUrl,
		sellUrl: sellUrl,
	}
}

func (b *bitonicAgent) RequestPrice(request *PriceRequest) <-chan *PriceResponse {
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
				Request: *request,
				Error:   fmt.Sprintf("Bitonic said: %s", priceData.Error),
			}
		} else {
			response <- &PriceResponse{
				Request: *request,
				Btc:     priceData.Btc,
				Eur:     priceData.Eur,
				Price:   priceData.Price,
			}
		}
	}()

	return response
}

func (b *bitonicAgent) buildRequestUrl(request *PriceRequest) (string, error) {
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
