package main

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/resc/rescbits/bitbot/bitonic"
	"github.com/resc/rescbits/bitbot/datastore"
	"github.com/resc/rescbits/bitbot/env"
	"github.com/resc/rescbits/bitbot/migrations"
	"github.com/resc/slack"
	log "github.com/sirupsen/logrus"
	"github.com/resc/rescbits/bitbot/bus"
)

// environment variables
const (
	BITBOT_SLACK_API_TOKEN = "BITBOT_SLACK_API_TOKEN"
	BITBOT_DATABASE_URL    = "BITBOT_DATABASE_URL"

	BITBOT_BITONIC_BUY_URL       = "BITBOT_BITONIC_BUY_URL"
	BITBOT_POLL_INTERVAL_SEC     = "BITBOT_POLL_INTERVAL_SEC"
	BITBOT_BITONIC_SELL_URL      = "BITBOT_BITONIC_SELL_URL"
	BITBOT_DATABASE_AUTO_MIGRATE = "BITBOT_DATABASE_AUTO_MIGRATE"
	BITBOT_SLACK_API_DEBUG       = "BITBOT_SLACK_API_DEBUG"
)

func main() {
	run()
}

func run() {
	shutdown := make(chan struct{})

	defer func() {
		close(shutdown)
		err := recover()
		if err != nil {
			log.Fatal(err)
		} else {
			log.Infof("stopped")
		}
	}()

	log.SetLevel(log.DebugLevel)

	env.Required(BITBOT_SLACK_API_TOKEN, "The slack.com api access token")
	env.Required(BITBOT_DATABASE_URL, "The postgres database uri e.g. postgres://user:password@localhost/dbname?application_name=bitbot&sslmode=disable")

	env.Optional(BITBOT_BITONIC_BUY_URL, "https://bitonic.nl/api/buy", "")
	env.Optional(BITBOT_BITONIC_SELL_URL, "https://bitonic.nl/api/sell", "")
	env.OptionalInt(BITBOT_POLL_INTERVAL_SEC, 30, "the bitonic poll interval (min= 10sec)")
	env.OptionalBool(BITBOT_DATABASE_AUTO_MIGRATE, false, "set this variable to true if the database schema should be auto-migrated on startup")
	env.OptionalBool(BITBOT_SLACK_API_DEBUG, false, "set this variable to true if the slack api library debug logging should be turned on")

	env.MustParse()

	// database schema check
	connectionString := env.String(BITBOT_DATABASE_URL)

	m, err := migrations.New(connectionString)
	if err != nil {
		panicIf(errors.Wrapf(err, "error initializing database migrations"))
	}

	if err := m.Ping(); err != nil {
		panicIf(errors.Wrapf(err, "error connecting to the database "))
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

	bot, err := newBot(rtm, "", bitonicApi, ds)
	panicIf(err)

	// samples
	samples := make(chan datastore.PriceSample)

	// spawn bitonic price poller
	pollInterval := time.Duration(env.Int(BITBOT_POLL_INTERVAL_SEC)) * time.Second
	go bitonic.PricePoller(bitonicApi, ds, pollInterval, shutdown, samples)
	RunNewPriceAlerter(updatedTriggers, samples, shutdown)
	bot.Listen()

	commands := bus.CreateTopic("commands")
	priceSamples := bus.CreateTopic("priceSamples")
	alerts := bus.CreateTopic("alerts")

	go persistPriceSamples(priceSamples, ds)

	triggerAlerts := priceSamples.Subscribe("triggerAlerts")

}

func persistPriceSamples(priceSamples bus.Topic, ds datastore.DataStore) {
	subscription := priceSamples.Subscribe("persistPriceSamples")
	for msg := range subscription.Messages() {
		switch m := msg.(type) {
		case datastore.PriceSample:
			if ouw, err := ds.StartUow(); err != nil {
				// TODO handle error
			} else {
				if err := ouw.SavePriceSamples(m); err != nil {
					// TODO handle error
				} else {
					if err := ouw.Commit(); err != nil {
						// TODO handle error
					}
				}
			}
		}
	}
}

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}
