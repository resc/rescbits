package bus

import (
	"sync"
	log "github.com/sirupsen/logrus"
)

type (
	topic struct {
		closeChannel sync.Once

		name string

		messages        chan interface{}
		addSubscription chan *subscription
		delSubscription chan *subscription
	}
)

func (t *topic) Name() string {
	return t.name
}

func (t *topic) Publish(m interface{}) {
	t.messages <- m
}

func (t *topic) Subscribe(name string) Subscription {
	return t.SubscribeBuffered(name, 0)
}

func (t *topic) SubscribeBuffered(name string, bufferDepth int) Subscription {
	log.Debugf("adding subscription %s to topic %s", name, t.Name())
	s := &subscription{
		name:     name,
		topic:    t,
		messages: make(chan interface{}, bufferDepth),
	}

	t.addSubscription <- s
	return s
}

func (t *topic) Close() error {
	t.closeChannel.Do(func() {
		close(t.messages)
	})
	return nil
}

func (t *topic) dispatcher() {
	defer log.Infof("dispatcher for topic %s stopped", t.Name())

	subs := make([]*subscription, 0, 16)

	msg := t.messages
	add := t.addSubscription
	del := t.delSubscription

	for {
		select {
		case m, ok := <-msg:
			if ok {
				for i := range subs {
					subs[i].messages <- m
				}
			} else {
				if len(subs) == 0 {
					return // immediately
				} else {
					// block add and msg channels permanently
					add = nil
					msg = nil
					// because add and msg channels are nil, only the del channel will be able to receive
					// so we are ready to go close all subscriptions now.
					for i := range subs {
						log.Debugf("closing subscription %s from closed topic %s", subs[i].Name(), t.Name())
						go subs[i].Close()
					}
				}
			}
		case s := <-add:
			subs = append(subs, s)
			log.Debugf("added subscription %s to topic %s", s.Name(), t.Name())

		case s := <-del:
			for i := range subs {
				if subs[i] == s {
					subs[i] = subs[len(subs)-1]
					subs[len(subs)-1] = nil
					subs = subs[:len(subs)-1]

					close(s.messages)
					log.Debugf("removed subscription %s from topic %s", s.Name(), t.Name())
				}
			}

			if add == nil && len(subs) == 0 {
				// aaand we're done...
				return
			}
		}
	}
}
