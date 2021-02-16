package queue_test

import (
	"context"
	"fmt"
	"github.com/DoNewsCode/std/pkg/contract"
	"github.com/DoNewsCode/std/pkg/core"
	"github.com/DoNewsCode/std/pkg/events"
	"github.com/DoNewsCode/std/pkg/queue"
	"github.com/DoNewsCode/std/pkg/queue/modqueue"
	"github.com/go-redis/redis/v8"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/providers/rawbytes"
	"github.com/oklog/run"
	"time"
)

type MockFactoryData struct {
	Value string
}

type MockFactoryListener struct{}

func (m MockFactoryListener) Listen() []contract.Event {
	return events.From(MockFactoryData{})
}

func (m MockFactoryListener) Process(_ context.Context, event contract.Event) error {
	fmt.Println(event.Data().(MockFactoryData).Value)
	return nil
}

// bootstrap is normally done when bootstrapping the framework. We mimic it here for demonstration.
func bootstrapFactories() *core.C {
	const sampleConfig = "{\"log\":{\"level\":\"error\"},\"queue\":{\"default\":{\"parallelism\":1},\"MyQueue\":{\"parallelism\":1}}}"

	// Make sure redis is running at localhost:6379
	c := core.New(
		core.WithConfigStack(rawbytes.Provider([]byte(sampleConfig)), json.Parser()),
	)

	// Add Provider
	c.AddCoreDependencies()
	c.AddDependency(modqueue.ProvideDispatcher)
	c.AddDependency(func() redis.UniversalClient {
		client := redis.NewUniversalClient(&redis.UniversalOptions{})
		_, _ = client.FlushAll(context.Background()).Result()
		return client
	})
	return c
}

// serve normally lives at serve command. We mimic it here for demonstration.
func serveFactories(c *core.C, duration time.Duration) {
	var g run.Group

	for _, r := range c.GetRunProviders() {
		r(&g)
	}

	// cancel the run group after some time, so that the program ends. In real project, this is not necessary.
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()
	g.Add(func() error {
		<-ctx.Done()
		return nil
	}, func(err error) {
		cancel()
	})

	err := g.Run()
	if err != nil {
		panic(err)
	}
}

func Example_factory() {
	c := bootstrapFactories()

	err := c.Invoke(func(maker modqueue.DispatcherMaker) {
		dispatcher, err := maker.Make("MyQueue")
		if err != nil {
			panic(err)
		}
		// Subscribe
		dispatcher.Subscribe(MockFactoryListener{})

		// Trigger an event
		evt := events.Of(MockFactoryData{Value: "hello world"})
		_ = dispatcher.Dispatch(context.Background(), queue.Persist(evt))
	})
	if err != nil {
		panic(err)
	}

	serveFactories(c, time.Second)

	// Output:
	// hello world
}
