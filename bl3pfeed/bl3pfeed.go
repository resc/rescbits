package bl3pfeed

import (
	"net/http"
)

type (
	Feed interface {
		BaseUrl() string
		Version() string
		Market() string
		Channel() string
		Open(http.Header) error
		Close() error

		// SetDebug to true to log the raw websocket messages instead of processing the messages
		SetDebug(bool)
	}

	FeedListener interface {
		FeedClosed(channel string, err error)
	}

	TradesFeedListener interface {
		FeedListener
		OnTrade(t *Trade)
	}

	OrderBookFeedListener interface {
		FeedListener
		OnOrderBookChanged(o *OrderBook)
	}
)
