package main

import (
	"bytes"
	"fmt"
	"github.com/resc/rescbits/bitbot/bitonic"
	"github.com/resc/rescbits/bitbot/datastore"
	"github.com/resc/rescbits/bitbot/plots"
	"github.com/resc/rescbits/bitbot/tools"
	log "github.com/sirupsen/logrus"
	"strconv"
	"strings"
	"time"
)

const (
	HelloCommand    = "hello"
	HelpCommand     = "help"
	BuyCommand      = "buy"
	SellCommand     = "sell"
	ShowCommand     = "show"
	LogLevelCommand = "loglevel"
)

var (
	helpTexts = map[string]string{
		BuyCommand:      "*buy [amount] [currency]*: Get a price quote for buying the given amount of the given currency (btc or eur)",
		SellCommand:     "*sell [amount] [currency]*: Get a price quote for selling the given amount of the given currency (btc or eur)",
		HelpCommand:     "*help*: list all commands",
		HelloCommand:    "*hello*: test if the bot responds",
		ShowCommand:     "*show*: shows a price graph of the last hour",
		LogLevelCommand: fmt.Sprintf("*loglevel [%s]*: sets the loglevel", strings.Join(getLogLevels(), "|")),
	}

	commands = map[string]command{
		HelloCommand:    SayHello,
		HelpCommand:     SayHelp,
		BuyCommand:      SayBuy,
		SellCommand:     SaySell,
		ShowCommand:     SayShow,
		LogLevelCommand: SetLogLevel,
	}
)

func SetLogLevel(c *conversation, parameters []string) (*conversation, string, error) {
	switch len(parameters) {
	case 0:
		return c, fmt.Sprintf("Current loglevel is *%s*", log.GetLevel().String()), nil
	case 1:
		// this is good
		break
	default:
		return c, "I didn't understand that\nHere's how the loglevel command works:\n" + helpTexts[LogLevelCommand], nil
	}

	level := strings.ToLower(parameters[0])
	switch level {

	case "debug":
		log.SetLevel(log.DebugLevel)
	case "info":
		log.SetLevel(log.InfoLevel)
	case "warn":
		fallthrough
	case "warning":
		log.SetLevel(log.WarnLevel)
	case "error":
		log.SetLevel(log.ErrorLevel)
	case "fatal":
		log.SetLevel(log.FatalLevel)
	case "panic":
		log.SetLevel(log.PanicLevel)
	default:
		loglevels := getLogLevels()
		return c, fmt.Sprintf("Invalid loglevel, choose one of *%s*", strings.Join(loglevels, "*, *")), nil
	}

	return c, fmt.Sprintf("Set loglevel to *%s*", log.GetLevel().String()), nil
}

func getLogLevels() []string {
	names := make([]string, 0, len(log.AllLevels))
	for i := range log.AllLevels {
		names = append(names, log.AllLevels[i].String())
	}
	return names
}

func SayHello(c *conversation, args []string) (*conversation, string, error) {
	userInfo, _ := c.bot.rtm.GetUserInfo(c.userID)
	txt := fmt.Sprintf("Hello to you too, %s", userInfo.Name)
	return c, txt, nil
}

func SayHelp(c *conversation, args []string) (*conversation, string, error) {
	sm := tools.NewSortedMapFromStringMap(helpTexts)
	txt := "*Commands:*\n"
	for _, kv := range *sm {
		txt += fmt.Sprintf("*%s*: %v\n", kv.Key, kv.Value)
	}

	return c, txt, nil
}

func SayIDontKnow(c *conversation, args []string) (*conversation, string, error) {
	_, txt, _ := SayHelp(c, args)
	return c, fmt.Sprintf("I don't know this '%s' you're speaking of...\n%s", args[0], txt), nil
}

func SayBuy(c *conversation, parameters []string) (*conversation, string, error) {
	if len(parameters) != 2 {
		return c, "I didn't understand that\nHere's how the buy command works:\n" + helpTexts[BuyCommand], nil
	}
	amount, err := strconv.ParseFloat(parameters[0], 64)
	if err != nil {
		return c, "The amount should be a number like 1.23\n" +
			err.Error() + "\n" +
			"Here's how the buy command works:\n" + helpTexts[BuyCommand], nil
	}

	response := <-c.bot.agent.RequestPrice(&bitonic.PriceRequest{
		Action:   bitonic.ActionBuy,
		Amount:   amount,
		Currency: bitonic.ParseCurrency(parameters[1]),
	})

	if response.Error == "" {
		price := fmt.Sprintf("%.2f", response.Price)

		return c, fmt.Sprintf("The buying price is %.2f EUR for %f BTC ( %s EUR/BTC )\n https://bitonic.nl/#buy", response.Eur, response.Btc, price), nil
	} else {
		return c, response.Error, nil
	}

}

func SayShow(c *conversation, parameters []string) (*conversation, string, error) {
	if len(parameters) != 2 {
		return c, "I didn't understand that\nHere's how the sell command works:\n" + helpTexts[ShowCommand], nil
	}
	if uow, err := c.bot.ds.StartUow(); err != nil {
		return c, "", err
	} else {
		c.sendTyping()
		end := time.Now()
		begin := time.Now().Add(-time.Hour)
		offset := begin
		maxResults := 500 // no unbounded result sets
		var allSamples []datastore.PriceSample
		var samples []datastore.PriceSample
		total, err := maxResults+1, error(nil)
		for total > maxResults {
			samples, total, err = uow.LoadPriceSamples(offset, end, maxResults)
			if err != nil {
				return c, "", err
			}
			allSamples = append(allSamples, samples...)
			if total > maxResults {
				offset = samples[len(samples)-1].Timestamp.Add(1)
			}
		}

		if png, err := plots.Render(allSamples); err != nil {
			return c, "Error while rendering graph: " + err.Error(), nil
		} else {
			title := fmt.Sprintf("Buy and Sell prices from %s to %s", begin.Format("Jan 2 15:04"), end.Format("15:04"))
			name := fmt.Sprintf("")
			c.bot.UploadImage(c.channelID, title, name, bytes.NewBuffer(png))
		}
	}
	return c, "Ehm, this command is a work in progress, sorry...", nil
}

func SaySell(c *conversation, parameters []string) (*conversation, string, error) {
	if len(parameters) != 2 {
		return c, "I didn't understand that\nHere's how the sell command works:\n" + helpTexts[SellCommand], nil
	}
	amount, err := strconv.ParseFloat(parameters[0], 64)
	if err != nil {
		return c, "The amount should be a number like 1.23\n" +
			err.Error() + "\n" +
			"Here's how the sell command works:\n" + helpTexts[SellCommand], nil
	}

	response := <-c.bot.agent.RequestPrice(&bitonic.PriceRequest{
		Action:   bitonic.ActionSell,
		Amount:   amount,
		Currency: bitonic.ParseCurrency(parameters[1]),
	})

	if response.Error == "" {
		price := fmt.Sprintf("%.2f", response.Price)

		return c, fmt.Sprintf("The selling price is %.2f EUR for %f BTC ( %s EUR/BTC )\n https://bitonic.nl/#sell", response.Eur, response.Btc, price), nil
	} else {
		return c, response.Error, nil
	}
}
