package main

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/resc/slack"
	"github.com/pkg/errors"
	"github.com/resc/rescbits/bitbot/datastore"
	"github.com/resc/rescbits/bitbot/migrations"
	"github.com/resc/rescbits/bitbot/env"
	"github.com/resc/rescbits/bitbot/bitonic"
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
	log.SetLevel(log.DebugLevel)

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

	shutdown := make(chan struct{})
	defer close(shutdown)

	// database schema check
	m, err := migrations.New(env.String(BITBOT_DATABASE_URL))
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

	// bitonic slackApi initialization
	buyUrl := env.String(BITBOT_BITONIC_BUY_URL)
	sellUrl := env.String(BITBOT_BITONIC_SELL_URL)
	bitonicApi := bitonic.New(buyUrl, sellUrl)

	// slack slackApi initialization
	slackApi := slack.New(env.String(BITBOT_SLACK_API_TOKEN))
	if env.Bool(BITBOT_SLACK_API_DEBUG) {
		log.Infof("Running bot in debug mode.")
		slackApi.SetDebug(true)
	}

	rtm := slackApi.NewRTMWithOptions(&slack.RTMOptions{
		UseRTMStart: false,
	})

	// spawn slack web socket message loop
	go rtm.ManageConnection()

	// spawn bitonic price poller
	go bitonic.PricePoller(bitonicApi, ds, 5*time.Second, shutdown)

	processSlackMessages(rtm, bitonicApi, ds)
}

func processSlackMessages(rtm *slack.RTM, bitonicApi *bitonic.Api, ds datastore.DataStore) {
	// bot initialization
	bot, err := newBot(rtm, "", bitonicApi, ds)
	panicIf(err)
	// run slack bot message loop
	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {

			case *slack.HelloEvent:
				log.Debug("Hello received")
			case *slack.ConnectedEvent:
				if bot, err = newBot(rtm, ev.Info.User.ID, bitonicApi, ds); err != nil {
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

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
