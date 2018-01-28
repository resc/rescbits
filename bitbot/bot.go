package main

import (
	"fmt"
	"github.com/resc/rescbits/bitbot/bitonic"
	"github.com/resc/rescbits/bitbot/datastore"
	"github.com/resc/slack"
	"io"
	"strings"

	log "github.com/sirupsen/logrus"
	"sync"
)

type bot struct {
	shutdown        sync.Once
	shutdownChannel chan struct{}

	// the slack user id for the bot.
	ID string
	// Tag is <@bot.ID>
	Tag           string
	rtm           *slack.RTM
	agent         *bitonic.Api
	ds            datastore.DataStore
	conversations map[string]*conversation
}

func newBot(rtm *slack.RTM, userID string, agent *bitonic.Api, ds datastore.DataStore) (*bot, error) {
	b := &bot{
		shutdownChannel: make(chan struct{}),

		ID:            userID,
		Tag:           "<@" + userID + ">",
		rtm:           rtm,
		agent:         agent,
		ds:            ds,
		conversations: make(map[string]*conversation),
	}

	return b, nil
}

func (bot *bot) Close() error {
	bot.shutdown.Do(func() {
		close(bot.shutdownChannel)
	})
	return nil
}

func (bot *bot) Listen() {
	for {
		select {
		case <-bot.shutdownChannel:
			log.Info("Shutting down...")
			return
		case msg := <-bot.rtm.IncomingEvents:
			switch ev := msg.Data.(type) {

			case *slack.HelloEvent:
				log.Debug("Hello received")
			case *slack.ConnectedEvent:
				bot.ID = ev.Info.User.ID
				log.Debugf("Connected: bot id is %s", bot.ID)
			case *slack.MessageEvent:
				// only respond to messages sent to me by others on the same channel:
				if bot.isMessageForMe(ev) {
					log.Infof("Message received: %+v", ev)
					bot.HandleMessage(ev)
				} else{
					log.Debugf("Message ignored: %+v", ev)
				}
			case *slack.ChannelJoinedEvent:
				bot.sendMessagef(ev.Channel.ID, "Hi all, thanks for inviting me to #%s", ev.Channel.Name)
			case *slack.RTMError:
				log.Errorf("Error: %+v\n", ev)
			case *slack.ConnectionErrorEvent:
				log.Errorf("Connection Error: %+v\n", ev)
			case *slack.AckErrorEvent:
				log.Errorf("Ack Error: %+v\n", ev)
			case *slack.InvalidAuthEvent:
				log.Errorf("Invalid credentials")
			case *slack.ConnectingEvent:
				log.Infof("Connecting...  Attempt: %d, ConnectionCount: %d ", ev.Attempt, ev.ConnectionCount)
			case *slack.DisconnectedEvent:
				log.Infof("Disconnected: %+v", ev)
			default:
				log.Debugf("%s: %+v", msg.Type, msg.Data)
			}
		}
	}
}

func (bot *bot) isDirectChannel(channelID string) bool {
	return strings.HasPrefix(channelID, "D")
}

func (bot *bot) isMessageForMe(ev *slack.MessageEvent) bool {
	// don't respond to myself
	if ev.Msg.User == bot.ID {
		return false
	}
	// don't respond to non-messages, edits or deletes
	if ev.Msg.Type != "message" || ev.Msg.Hidden || ev.Msg.Username == "slackbot" {
		return false
	}
	// respond to all messages on a direct channel
	if bot.isDirectChannel(ev.Channel) {
		return true
	}

	// or if i'm @mentioned in a channel
	return strings.Contains(ev.Msg.Text, bot.Tag)
}
func (bot *bot) UploadImage(channelID string, title string, name string, image io.Reader) {
	panic("not implemented")
}

func (bot *bot) sendMessagef(channel string, format string, args ...interface{}) error {
	return bot.sendMessage(channel, fmt.Sprintf(format, args...))
}

func (bot *bot) sendMessage(channelID string, text string) error {
	msg := bot.rtm.NewOutgoingMessage(text, channelID)
	bot.rtm.SendMessage(msg)
	return nil
}

func (bot *bot) sendTyping(channelID string) error {
	msg := bot.rtm.NewTypingMessage(channelID)
	bot.rtm.SendMessage(msg)
	return nil
}

func (bot *bot) HandleMessage(ev *slack.MessageEvent) {
	c := bot.getOrCreateConversation(ev.Msg.User, ev.Channel)
	c.HandleMessage(ev)
}

func (bot *bot) getOrCreateConversation(userID, channelID string) *conversation {
	if c, ok := bot.conversations[userID]; ok && c.channelID == channelID {
		return c
	} else {
		c = bot.newConversation(userID, channelID)
		return c
	}
}

func (bot *bot) newConversation(userID, channelID string) *conversation {
	c := newConversation(bot, userID, channelID, nil, nil)
	bot.setConversation(userID, c)
	return c
}

func (bot *bot) setConversation(userID string, c *conversation) {
	if c == nil {
		delete(bot.conversations, userID)
	} else {
		bot.conversations[userID] = c
	}
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
