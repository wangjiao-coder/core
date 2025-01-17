package leader

import (
	"context"
	"fmt"

	"github.com/DoNewsCode/core/config"
	"github.com/DoNewsCode/core/contract"
	"github.com/DoNewsCode/core/di"
	"github.com/DoNewsCode/core/key"
	leaderetcd2 "github.com/DoNewsCode/core/leader/leaderetcd"
	"github.com/DoNewsCode/core/otetcd"
	"github.com/oklog/run"
)

/*
Providers returns a set of dependency providers for *Election and *Status.
	Depends On:
		contract.AppName
		contract.Env
		contract.ConfigAccessor
		contract.Dispatcher
		Driver       `optional:"true"`
		otetcd.Maker `optional:"true"`
	Provide:
		Election *Election
		Status   *Status
*/
func Providers() di.Deps {
	return []interface{}{provide}
}

type in struct {
	di.In

	AppName    contract.AppName
	Env        contract.Env
	Config     contract.ConfigUnmarshaler
	Dispatcher contract.Dispatcher
	Driver     Driver       `optional:"true"`
	Maker      otetcd.Maker `optional:"true"`
}

type out struct {
	di.Out

	Election *Election
	Status   *Status
}

func provide(in in) (out, error) {
	if err := determineDriver(&in); err != nil {
		return out{}, err
	}
	e := NewElection(in.Dispatcher, in.Driver)
	return out{
		Election: e,
		Status:   e.status,
	}, nil
}

// Module marks out as a module.
func (m out) Module() interface{} { return m }

func (m out) ProvideRunGroup(group *run.Group) {
	ctx, cancel := context.WithCancel(context.Background())
	group.Add(func() error {
		err := m.Election.Campaign(ctx)
		if err != nil {
			return err
		}
		<-ctx.Done()
		return nil
	}, func(err error) {
		_ = m.Election.Resign(ctx)
		cancel()
	})
}

func determineDriver(in *in) error {
	var option Option
	if in.Driver == nil {
		if err := in.Config.Unmarshal("leader", &option); err != nil {
			return fmt.Errorf("leader election configuration error: %w", err)
		}
		if option.EtcdName == "" {
			option.EtcdName = "default"
		}
		if in.Maker == nil {
			return fmt.Errorf("must provider an otetcd.Maker or provider a leader.Driver")
		}
		etcdClient, err := in.Maker.Make(option.EtcdName)
		if err != nil {
			return fmt.Errorf("failed to initiate leader election with etcd driver (%s): %w", option.EtcdName, err)
		}
		in.Driver = leaderetcd2.NewEtcdDriver(etcdClient, key.New(in.AppName.String(), in.Env.String()))
	}
	return nil
}

type configOut struct {
	di.Out

	Config []config.ExportedConfig `group:"config,flatten"`
}

func provideConfig() configOut {
	return configOut{Config: []config.ExportedConfig{
		{
			Owner: "leader",
			Data: map[string]interface{}{
				"leader": map[string]interface{}{
					"etcdName": "default",
				},
			},
			Comment: "The leader election config",
		},
	}}
}
