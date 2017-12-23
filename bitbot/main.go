package main

import (
	"os"
	"strings"
	"github.com/resc/slack"
	log "github.com/sirupsen/logrus"
	"fmt"
	"strconv"
)

// environment variables
const (
	BITBOT_SLACK_API_TOKEN = "BITBOT_SLACK_API_TOKEN"
	BITONIC_BUY_URL        = "BITONIC_BUY_URL"
	BITONIC_SELL_URL       = "BITONIC_SELL_URL"
	BITBOT_SLACK_API_DEBUG = "BITBOT_SLACK_API_DEBUG"
)

func main() {
	env := getEnvironment(true, BITBOT_SLACK_API_TOKEN, BITONIC_BUY_URL, BITONIC_SELL_URL)
	opt := getEnvironment(false, BITBOT_SLACK_API_DEBUG)
	log.SetLevel(log.DebugLevel)

	api := slack.New(env[BITBOT_SLACK_API_TOKEN])
	bitonic := NewBitonicAgent(env[BITONIC_BUY_URL], env[BITONIC_SELL_URL])

	if debug, ok := opt[BITBOT_SLACK_API_DEBUG]; ok {
		mode := debug == "" || strings.ToLower(debug) == "true"
		if mode {
			log.Infof("Running bot in debug mode.")
		}
		api.SetDebug(mode)
	}

	rtm := api.NewRTMWithOptions(&slack.RTMOptions{
		UseRTMStart: false,
	})
	go rtm.ManageConnection() // spawn slack bot
	bot, err := newBot(rtm, "", bitonic)
	panicIf(err)

	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {

			case *slack.HelloEvent:
				log.Debug("Hello received")
			case *slack.ConnectedEvent:
				if bot, err = newBot(rtm, ev.Info.User.ID, bitonic); err != nil {
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

type bot struct {
	// the slack user id for the bot.
	ID string
	// Tag is <@bot.ID>
	Tag           string
	rtm           *slack.RTM
	agent         *bitonicAgent
	conversations map[string]*conversation
	channels      []slack.Channel
}

func newBot(rtm *slack.RTM, userID string, agent *bitonicAgent) (*bot, error) {
	b := &bot{
		ID:            userID,
		Tag:           "<@" + userID + ">",
		rtm:           rtm,
		agent:         agent,
		conversations: make(map[string]*conversation),
	}

	return b, nil;
}

func (bot *bot) isDirectChannel(channelID string) bool {
	return strings.HasPrefix(channelID, "D")
}

func (bot *bot) isMessageForMe(ev *slack.MessageEvent) bool {
	// don't respond to myself
	if ev.Msg.User == bot.ID {
		return false;
	}
	// don't respond to non-messages, edits or deletes
	if ev.Msg.Type != "message" || ev.Msg.Hidden || ev.Msg.Username =="slackbot" {
		return false;
	}
	// respond to all messages on a direct channel
	if bot.isDirectChannel(ev.Channel) {
		return true
	}

	// or if i'm mentioned in a channel
	return strings.Contains(ev.Msg.Text, bot.Tag)
}

func (bot *bot) sendMessagef(channel string, format string, args ...interface{}) error {
	return bot.sendMessage(channel, fmt.Sprintf(format, args...))
}

func (bot *bot) sendMessage(channelID string, text string) error {
	msg := bot.rtm.NewOutgoingMessage(text, channelID)
	bot.rtm.SendMessage(msg)
	return nil
}

func (bot *bot) HandleMessage(ev *slack.MessageEvent) {
	c := bot.getConversationForUser(ev.Msg.User)
	c.HandleMessage(ev)
}

func (bot *bot) getConversationForUser(userID string) *conversation {
	if c, ok := bot.conversations[userID]; ok {
		return c
	} else {
		c = bot.newConversation(userID)
		return c
	}
}

func (bot *bot) newConversation(userID string) *conversation {
	c := &conversation{
		bot:    bot,
		userID: userID,
		rtm:    bot.rtm,
		ctx:    "new",
	}
	// just kill the old conversation if there's one
	bot.conversations[userID] = c
	return c
}

func (bot *bot) stripMyNameAndSpaces(msg string) string {
	i := strings.Index(msg, bot.Tag)
	if i >= 0 {
		msg = msg[i+len(bot.Tag):]
	}

	msg = strings.Replace(msg, bot.Tag, "", -1)
	msg = strings.TrimSpace(msg)
	return msg
}

func (bot *bot) UpdateJoinedChannels() error {
	if channels, err := bot.rtm.GetChannels(true); err != nil {
		return err
	} else {
		joinedChannels := make([]slack.Channel, 0)
		for _, c := range channels {
			log.Debugf("%+v", c)
			if c.IsMember {
				joinedChannels = append(joinedChannels, c)
			}
		}
		bot.channels = joinedChannels
		return nil
	}
}
func (bot *bot) HasJoinedChannel(channel string) bool {
	channel = strings.TrimPrefix(channel, "#")
	for _, c := range bot.channels {
		if c.ID == channel || c.Name == channel {
			return true
		}
	}
	return false
}

type conversation struct {
	bot    *bot
	userID string
	ctx    string
	rtm    *slack.RTM
}

const (
	buyHelpText  = "*buy [amount] [currency]*: Get a price quote for buying the given amount of the given currency (btc or eur)"
	sellHelpText = "*sell [amount] [currency]*: Get a price quote for selling the given amount of the given currency (btc or eur)"
)

func (c *conversation) HandleMessage(ev *slack.MessageEvent) {
	msg := c.bot.stripMyNameAndSpaces(ev.Msg.Text)
	userInfo, _ := c.rtm.GetUserInfo(ev.Msg.User)
	log.Debugf("%+v", ev)
	commandAndParameters := strings.Split(msg, " ")
	txt, err := "", (error)(nil)
	if len(commandAndParameters) < 1 {

	} else {
		cmd := strings.ToLower(commandAndParameters[0])
		parameters := commandAndParameters[1:]
		switch cmd {
		case "hello":
			txt += fmt.Sprintf("Hello to you too, %s", userInfo.Name)

		case "buy":
			txt, err = c.HandleBuy(parameters)
		case "sell":
			txt, err = c.HandleSell(parameters)
		default:
			txt += fmt.Sprintf( "I don't know this '%s' you're speaking of...\n", cmd)
			fallthrough
		case "help":
			txt += "*Commands:*\n" +
				"*hello*: test if the bot responds\n" +
				buyHelpText + "\n" +
				sellHelpText + "\n"
		}
	}
	if err != nil {
		txt = "Something failed, please try again:  " + err.Error()
	}
	outMsg := c.rtm.NewOutgoingMessage(txt, ev.Channel)
	c.rtm.SendMessage(outMsg)
}
func (c *conversation) HandleBuy(parameters []string) (string, error) {
	if len(parameters) != 2 {
		return "I didn't understand that\nHere's how the buy command works:\n" + buyHelpText, nil
	}
	amount, err := strconv.ParseFloat(parameters[0], 64)
	if err != nil {
		return "The amount should be a number like 1.23\n" +
			err.Error() + "\n" +
			"Here's how the buy command works:\n" + buyHelpText, nil
	}

	response := <-c.bot.agent.RequestPrice(&PriceRequest{
		Action:   ActionBuy,
		Amount:   amount,
		Currency: parameters[1],
	})

	if response.Error == "" {
		price:= fmt.Sprintf("%.2f",response.Price)

		return fmt.Sprintf("The buying price is %.2f EUR for %f BTC ( %s EUR/BTC )\n https://bitonic.nl/#buy", response.Eur, response.Btc, price), nil
	} else {
		return response.Error, nil
	}

}

func (c *conversation) HandleSell(parameters []string) (string, error) {
	if len(parameters) != 2 {
		return "I didn't understand that\nHere's how the sell command works:\n" + sellHelpText, nil
	}
	amount, err := strconv.ParseFloat(parameters[0], 64)
	if err != nil {
		return "The amount should be a number like 1.23\n" +
			err.Error() + "\n" +
			"Here's how the sell command works:\n" + buyHelpText, nil
	}

	response := <-c.bot.agent.RequestPrice(&PriceRequest{
		Action:   ActionSell,
		Amount:   amount,
		Currency: parameters[1],
	})

	if response.Error == "" {
		price:= fmt.Sprintf("%.2f",response.Price)

		return fmt.Sprintf("The selling price is %.2f EUR for %f BTC ( %s EUR/BTC )\n https://bitonic.nl/#sell", response.Eur, response.Btc, price), nil
	} else {
		return response.Error, nil
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
