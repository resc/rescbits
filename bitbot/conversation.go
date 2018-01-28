package main

import (
	"github.com/resc/slack"
	"strings"
	"github.com/sirupsen/logrus"
)

type (
	conversation struct {
		bot       *bot
		userID    string
		channelID string
		parent    *conversation
		commands  map[string]command
	}

	command func(c *conversation, args []string) (*conversation, string, error)
)

func newConversation(bot *bot, userID string, channelID string, cCommands map[string]command, parent *conversation) *conversation {
	if cCommands == nil {
		cCommands = commands
	}


	return &conversation{
		bot:       bot,
		userID:    userID,
		channelID: channelID,
		commands:  cCommands,
		parent:    parent,
	}
}

func (c *conversation) isRoot() bool {
	return c.parent == nil
}

func (c *conversation) getRoot() *conversation {
	x := c
	for !x.isRoot() {
		x = x.parent
	}
	return x
}

func (c *conversation) pushConversationContext(subCommands map[string]command) {
	next := newConversation(c.bot, c.userID, c.channelID, subCommands, c)
	c.bot.setConversation(c.userID, next)
}

func (c *conversation) popConversationContext(subCommands map[string]command) {
	if c.isRoot() {
		return
	}
	c.bot.setConversation(c.userID, c.parent)
}

func (c *conversation) HandleMessage(ev *slack.MessageEvent) {
	if ev.User != c.userID && ev.Channel != c.channelID {
		logrus.Debugf("Ignored message %s", ev.Name)
		return
	}
	c.sendTyping()
	msg := c.bot.stripMyNameAndSpaces(ev.Msg.Text)
	c.handleCommand(msg)
}

func (c *conversation) handleCommand(commandText string) {
	commandAndParameters := strings.Split(commandText, " ")
	nextConversation, txt, err := c, "", (error)(nil)
	if len(commandAndParameters) < 1 {
		c.getRoot().handleCommand("help")
		return
	} else {
		cmd := strings.ToLower(commandAndParameters[0])
		parameters := commandAndParameters[1:]
		if command, ok := c.commands[cmd]; ok {
			nextConversation, txt, err = command(c, parameters)
		} else {
			nextConversation, txt, err = SayIDontKnow(c, []string{cmd})
		}
	}

	if err != nil {
		txt = "Something went wrong, sorry! please try again.  " + err.Error()
	}

	c.bot.setConversation(c.userID, nextConversation)
	c.sendMessage(txt)
}

func (c *conversation) sendMessage(txt string) {
	c.bot.sendMessage(c.channelID, txt)
}

func (c *conversation) sendMessagef(txt string, args ...interface{}) {
	c.bot.sendMessagef(c.channelID, txt, args...)
}

func (c *conversation) sendTyping() {
	c.bot.sendTyping(c.channelID)
}
