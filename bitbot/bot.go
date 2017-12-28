package main

import (
	"strings"
	log "github.com/sirupsen/logrus"
	"github.com/resc/slack"
	"fmt"
	"github.com/resc/rescbits/bitbot/bitonic"
	"github.com/resc/rescbits/bitbot/datastore"
)

type bot struct {
	// the slack user id for the bot.
	ID string
	// Tag is <@bot.ID>
	Tag           string
	rtm           *slack.RTM
	agent         *bitonic.Api
	ds            datastore.DataStore
	conversations map[string]*conversation
	channels      []slack.Channel
}

func newBot(rtm *slack.RTM, userID string, agent *bitonic.Api, ds datastore.DataStore) (*bot, error) {
	b := &bot{
		ID:            userID,
		Tag:           "<@" + userID + ">",
		rtm:           rtm,
		agent:         agent,
		ds:            ds,
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
	if ev.Msg.Type != "message" || ev.Msg.Hidden || ev.Msg.Username == "slackbot" {
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
