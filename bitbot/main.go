package main

import (
	"os"
	"strings"
	log "github.com/sirupsen/logrus"

	"github.com/resc/slack"

	"github.com/resc/rescbits/bitbot/datastore"
	"github.com/resc/rescbits/bitbot/migrations"
	"github.com/resc/rescbits/bitbot/env"

	"github.com/pkg/errors"
	"fmt"
	"time"
)

// environment variables
const (
	BITBOT_SLACK_API_TOKEN = "BITBOT_SLACK_API_TOKEN"
	BITBOT_DATABASE_URL    = "BITBOT_DATABASE_URL"

	BITBOT_BITONIC_BUY_URL       = "BITBOT_BITONIC_BUY_URL"
	BITBOT_BITONIC_SELL_URL      = "BITBOT_BITONIC_SELL_URL"
	BITBOT_DATABASE_AUTO_MIGRATE = "BITBOT_DATABASE_AUTO_MIGRATE"
	BITBOT_SLACK_API_DEBUG       = "BITBOT_SLACK_API_DEBUG"
)

func main() {
	env.Required(BITBOT_SLACK_API_TOKEN, "The slack.com api access token")
	env.Required(BITBOT_DATABASE_URL, "The postgres database uri e.g. postgres://user:password@localhost/dbname?application_name=bitbot&sslmode=disable")

	env.Optional(BITBOT_BITONIC_BUY_URL, "https://bitonic.nl/api/buy", "")
	env.Optional(BITBOT_BITONIC_SELL_URL, "https://bitonic.nl/api/sell", "")
	env.OptionalBool(BITBOT_DATABASE_AUTO_MIGRATE, false, "set this variable to true if the database schema should be auto-migrated on startup")
	env.OptionalBool(BITBOT_SLACK_API_DEBUG, false, "set this variable to true if the slack api library debug logging should be turned on")

	env.MustParse()

	run()
}

func run() {
	log.SetLevel(log.DebugLevel)

	shutdown := make(chan struct{})
	defer close(shutdown)

	m, err := migrations.New(env.String(BITBOT_DATABASE_URL))
	// database schema check

	if err != nil {
		panicIf(errors.Wrap(err, "error initializing database migrations"))
	}

	if err := m.Ping(); err != nil {
		panicIf(errors.Wrap(err, "error connecting to the database"))
	}

	if env.Bool(BITBOT_DATABASE_AUTO_MIGRATE) {
		err := m.Migrate()
		panicIf(errors.Wrap(err, "error migrating database schema"))
	}

	if ok, err := m.IsUpToDate(); err != nil {
		panicIf(err)
	} else {
		if !ok {
			panicIf(fmt.Errorf("database schema not up to date, please run this app again with env var %s=true", BITBOT_DATABASE_AUTO_MIGRATE))
		}
	}

	// datastore initialization
	ds, err := datastore.Open(env.String(BITBOT_DATABASE_URL))
	panicIf(err)
	panicIf(ds.Ping())

	// bitonic api initialization
	bitonic := NewBitonicAgent(env.String(BITBOT_BITONIC_BUY_URL), env.String(BITBOT_BITONIC_SELL_URL))

	// slack api initialization
	api := slack.New(env.String(BITBOT_SLACK_API_TOKEN))

	if env.Bool(BITBOT_SLACK_API_DEBUG) {
		log.Infof("Running bot in debug mode.")
		api.SetDebug(true)
	}

	rtm := api.NewRTMWithOptions(&slack.RTMOptions{
		UseRTMStart: false,
	})

	// bot initialization
	bot, err := newBot(rtm, "", bitonic, ds)
	panicIf(err)

	// spawn slack web socket message loop
	go rtm.ManageConnection()

	// spawn bitonic price poller
	go bitonicPricePoller(bitonic, ds, 5*time.Second, shutdown)

	// run bot message loop
	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {

			case *slack.HelloEvent:
				log.Debug("Hello received")
			case *slack.ConnectedEvent:
				if bot, err = newBot(rtm, ev.Info.User.ID, bitonic, ds); err != nil {
					log.Errorf("Error connecting bot: %s", err.Error())
				} else {
					log.Debugf("Connected: bot id is %s", bot.ID)
				}
			case *slack.MessageEvent:
				// only respond to messages sent to me by others on the same channel:
				log.Debugf("Message received: %+v", ev)
				if bot.isMessageForMe(ev) {
					bot.HandleMessage(ev)
				}
			case *slack.ChannelJoinedEvent:
				bot.sendMessagef(ev.Channel.ID, "Hi all, thanks for inviting me to #%s", ev.Channel.Name)
			case *slack.PresenceChangeEvent:
				log.Debugf("Presence changed to %s for user %s (type %s)", ev.Presence, ev.User, ev.Type)
			case *slack.LatencyReport:
				log.Debugf("Current latency: %+v\n", ev.Value)
			case *slack.RTMError:
				log.Errorf("Error: %+v\n", ev)
			case *slack.ConnectionErrorEvent:
				log.Errorf("Connection Error: %+v\n", ev)
			case *slack.AckErrorEvent:
				log.Errorf("Ack Error: %+v\n", ev)
			case *slack.InvalidAuthEvent:
				log.Errorf("Invalid credentials")
			case *slack.ReconnectUrlEvent:
				log.Debugf("Reconnect url %s: %+v", msg.Type, msg.Data)
			case *slack.ConnectingEvent:
				log.Infof("Connecting...  Attempt: %d, ConnectionCount: %d ", ev.Attempt, ev.ConnectionCount)
			case *slack.DisconnectedEvent:
				log.Infof("Disconnected: %+v", ev)
			case *slack.UserChangeEvent:
				log.Debugf("%s: %+v", msg.Type, msg.Data)
			default:
				log.Debugf("%s: %+v", msg.Type, msg.Data)
			}
		}
	}
}

func bitonicPricePoller(bitonic *bitonicAgent, ds datastore.DataStore, pollInterval time.Duration, shutdown <-chan struct{}) {
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
				go bitonicPricePoller(bitonic, ds, pollInterval, shutdown)
			}
		}
	}()
	for {
		select {
		case <-shutdown:
			return
		case <-time.After(pollInterval):
			buyResponse := bitonic.RequestPrice(&PriceRequest{
				Amount:   1,
				Currency: CurrencyBtc,
				Action:   ActionBuy,
			})
			sellResponse := bitonic.RequestPrice(&PriceRequest{
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
						uow.SavePriceSamples(datastore.PriceSample{
							Type:      "B",
							Timestamp: buy.Time,
							Price:     int64(buy.Price * 1e5),
						})
					} else {
						log.Errorf("Error fetching buy price: %s", buy.Error)
					}

					sell := <-sellResponse
					if sell.Error == "" {
						uow.SavePriceSamples(datastore.PriceSample{
							Type:      "S",
							Timestamp: sell.Time,
							Price:     int64(sell.Price * 1e5),
						})
					} else {
						log.Errorf("Error fetching buy price: %s", sell.Error)
					}
				}()
			}
		}
	}
}

func getEnvironment(panicOnMissingKeys bool, keys ...string) map[string]string {
	missingKeys := make([]string, 0)
	env := make(map[string]string)
	for _, key := range keys {
		if value, ok := os.LookupEnv(key); ok {
			// the value may be an empty string, but that's ok,
			// the variable was present when ok == true
			env[key] = value
		} else {
			missingKeys = append(missingKeys, key)
		}
	}

	if panicOnMissingKeys && len(missingKeys) > 0 {
		panic("Missing environment variables: " + strings.Join(missingKeys, ", "))
	}
	return env
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
