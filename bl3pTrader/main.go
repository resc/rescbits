package main

import (
	"bufio"
	"fmt"
	"github.com/resc/rescbits/bl3pfeed"
	log "github.com/sirupsen/logrus"
	"os"
	"time"
)

func main() {

	baseUrl := "wss://api.bl3p.eu"
	version := "1"
	market := "BTCEUR"

	run(baseUrl, version, market)
}

func run(baseUrl string, version string, market string) {
	trades, err := runTrades(baseUrl, version, market)
	if err != nil {
		log.Fatal(err)
	}
	defer trades.Close()

	//orderBooks, err := runOrderBooks(baseUrl, version, market)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer orderBooks.Close()

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Press return to exit")
	reader.ReadString('\n')
}

type (
	tradePersister struct {
	}
)

func (t *tradePersister) FeedClosed(channel string, err error) {
}

func (t *tradePersister) OnTrade(trade *bl3pfeed.Trade) {
	amount := float64(trade.Amount) / 1e8
	price := float64(trade.Price) / 1e5
	log.Debugf("%s %4s %2.5f BC %5.2f EUR (%.5f) %s", trade.Marketplace, trade.Type, amount, price*amount, price, time.Unix(trade.Date, 0).String())
}

var _ bl3pfeed.TradesFeedListener = (*tradePersister)(nil)

func runTrades(baseUrl string, version string, market string) (bl3pfeed.Feed, error) {
	var listener = &tradePersister{}
	trades, err := bl3pfeed.NewTrades(baseUrl, version, market, listener)

	if err != nil {
		return nil, err
	}
	err = trades.Open(nil)
	if err != nil {
		return nil, err
	}

	return trades, nil
}

type (
	orderbookPersister struct {
	}
)

func (t *orderbookPersister) FeedClosed(channel string, err error) {
}

func (t *orderbookPersister) OnOrderBookChanged(o *bl3pfeed.OrderBook) {
	log.Print("Orderbook:")
	for _, ask := range o.Asks {
		amount := float64(ask.Amount) / 1e8
		price := float64(ask.Price) / 1e5
		log.Printf("\tASK: %.5f BC %.5f EUR", amount, price)
	}
	for _, bid := range o.Bids {
		amount := float64(bid.Amount) / 1e8
		price := float64(bid.Price) / 1e5
		log.Printf("\tBID: %.5f BC %.5f EUR", amount, price)
	}
}

func runOrderBooks(baseUrl string, version string, market string) (bl3pfeed.Feed, error) {
	listener := &orderbookPersister{}
	orders, err := bl3pfeed.NewOrderBook(baseUrl, version, market, listener)

	if err != nil {
		return nil, err
	}
	err = orders.Open(nil)
	if err != nil {
		return nil, err
	}

	return orders, nil
}
