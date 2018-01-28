package bus

import (
	"sync"
	"fmt"
	log "github.com/sirupsen/logrus"
)

type subscription struct {
	closeChannel sync.Once

	name     string
	topic    *topic
	messages chan interface{}
}

func (s *subscription) Name() string {
	return s.name
}

func (s *subscription) Topic() string {
	return s.topic.Name()
}

func (s *subscription) Messages() <-chan interface{} {
	return s.messages
}

func (s *subscription) Close() error {
	closed := false
	s.closeChannel.Do(func() {
		log.Debugf("removing subscription %s from topic %s", s.Name(), s.topic.Name())
		s.topic.delSubscription <- s
		closed = true
	})
	if closed {
		return nil
	} else {
		return fmt.Errorf("Already closed")
	}
}
