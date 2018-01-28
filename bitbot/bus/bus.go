package bus

type (
	Topic interface {
		Name() string
		Subscribe(name string) Subscription
		SubscribeBuffered(name string, bufferDepth int) Subscription
		Publish(m interface{})
		Close() error
	}

	Subscription interface {
		Name() string
		Topic() string
		Messages() <-chan interface{}
		Close() error
	}
)

func CreateTopic(topic string) Topic {
	return CreateTopicBuffered(topic, 0)
}

func CreateTopicBuffered(topic string, bufferDepth int) Topic {
	t := &topic{
		name:            topic,
		messages:        make(chan interface{}, bufferDepth),
		addSubscription: make(chan *subscription),
		delSubscription: make(chan *subscription),
	}
	go t.dispatcher()
	return t
}
