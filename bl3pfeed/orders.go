package bl3pfeed

import (
	"github.com/gorilla/websocket"
	"net/http"
	"sync"

	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
)

type (
	OrderBooks struct {
		close sync.Once

		baseUrl string
		version string
		market  string
		channel string

		conn     *websocket.Conn
		done     chan struct{}
		listener OrderBookFeedListener

		debug bool
		log   *log.Entry
	}

	OrderBook struct {
		Market string   `json:"marketplace"`
		Asks   []*Order `json:"asks"`
		Bids   []*Order `json:"bids"`
	}

	Order struct {
		Price  int `json:"price_int"`
		Amount int `json:"amount_int"`
	}

	OrderBookFunc func(*OrderBook)
)

var _ Feed = (*OrderBooks)(nil)

func NewOrderBook(baseUrl, version, market string, listener OrderBookFeedListener) (*OrderBooks, error) {
	if listener == nil {
		return nil, errors.New("No listener supplied")
	}
	channel := "orderbook"
	return &OrderBooks{
		baseUrl: baseUrl,
		version: version,
		market:  market,
		channel: channel,

		done:     make(chan struct{}),
		listener: listener,
		log: log.WithFields(log.Fields{
			"market":  market,
			"channel": channel,
		}),
	}, nil
}

func (o *OrderBooks) BaseUrl() string { return o.baseUrl }
func (o *OrderBooks) Version() string { return o.version }
func (o *OrderBooks) Market() string  { return o.market }
func (o *OrderBooks) Channel() string { return o.channel }

func (o *OrderBooks) SetDebug(b bool) { o.debug = b }

func (t *OrderBooks) Open(h http.Header) error {
	url := fmt.Sprintf("%s/%s/%s/%s", t.baseUrl, t.version, t.market, t.channel)
	log.Debugf("Dailing %s...", url)
	conn, resp, err := websocket.DefaultDialer.Dial(url, h)
	if err != nil {
		return err
	}

	log.Infof("Connected to %s (%d %s)", url, resp.StatusCode, resp.Status)

	t.conn = conn

	go t.receive()

	return nil
}

func (t *OrderBooks) Close() error {
	closed := false
	t.close.Do(func() {
		close(t.done)
		closed = true
	})
	if closed {
		return nil
	}
	return errors.New("Already closed")
}

func (t *OrderBooks) receive() {
	defer func() {
		err := recover()
		if err != nil {
			log.Errorf("receive: ", err)
		}
		err = t.conn.Close()

		if err != nil {
			log.Errorf("receive: ", err)
		}
	}()

	for {
		if t.debug {
			typ, bytes, err := t.conn.ReadMessage()
			if err != nil {
				log.Errorf("readConn: ", err)
				return
			}
			log.Infof("%d: %s", typ, string(bytes))
		} else {
			orderBook := &OrderBook{}
			err := t.conn.ReadJSON(orderBook)
			if err != nil {
				log.Errorf("readConn: ", err)
				return
			}
			t.listener.OnOrderBookChanged(orderBook)
		}

		select {
		case <-t.done:
			return
		default:
			break
		}
	}
}
