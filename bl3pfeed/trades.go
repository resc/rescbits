package bl3pfeed

import (
	"sync"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"strings"
	"net/http"
	"errors"
	"fmt"
)

type (
	Trades struct {
		close sync.Once

		baseUrl string
		version string
		market  string
		channel string

		conn     *websocket.Conn
		listener TradesFeedListener
		done     chan struct{}

		debug bool
		log   *log.Entry
	}

	Trade struct {
		Date        int64  `json:"date"`
		Marketplace string `json:"marketplace"`
		Price       int    `json:"price_int"`
		Type        string `json:"type"`
		Amount      int    `json:"amount_int"`
	}

	TradeFunc func(*Trade)
)

var _ Feed = (*Trades)(nil)

func NewTrades(baseUrl, version, market string, l TradesFeedListener) (*Trades, error) {
	if l == nil {
		return nil, errors.New("l TradesFeedListener is nil")
	}
	channel := "trades"
	return &Trades{
		baseUrl:  strings.TrimSuffix(baseUrl, "/"),
		version:  version,
		market:   market,
		channel: channel,
		listener: l,
		done:     make(chan struct{}),
		log: log.WithFields(log.Fields{
			"market":  market,
			"channel": channel,
		}),
	}, nil
}

func (t *Trades) BaseUrl() string { return t.baseUrl; }
func (t *Trades) Version() string { return t.version; }
func (t *Trades) Market() string  { return t.market; }
func (t *Trades) Channel() string { return t.channel; }
func (t *Trades) SetDebug(b bool) { t.debug = b }

func (t *Trades) Open(h http.Header) error {
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

func (t *Trades) Close() error {
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

func (t *Trades) receive() {
	defer func() {
		err := recover().(error)
		if err != nil {
			log.Error("receive: ", err)
		}
		err = t.conn.Close()

		if err != nil {
			log.Error("receive: ", err)
		}
		t.listener.FeedClosed(t.channel, err)
	}()

	for {
		if t.debug {
			typ, bytes, err := t.conn.ReadMessage()
			if err != nil {
				panic(err)
			}
			log.Infof("%d: %s", typ, string(bytes))
		} else {
			trade := &Trade{}
			err := t.conn.ReadJSON(trade)
			if err != nil {
				panic(err)
			}
			t.listener.OnTrade(trade)
		}

		select {
		case <-t.done:
			return
		default:
			break;
		}
	}
}
