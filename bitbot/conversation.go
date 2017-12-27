package main

import (
	log "github.com/sirupsen/logrus"
	"strings"
	"fmt"
	"strconv"
	"github.com/resc/slack"
)

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

