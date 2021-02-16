package config

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/DoNewsCode/std/pkg/contract"
	"github.com/ghodss/yaml"
	"github.com/oklog/run"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"path/filepath"
)

type Module struct {
	Conf      *KoanfAdapter
	Container contract.Container
}

func (m Module) ProvideRunGroup(group *run.Group) {
	if m.Conf.watcher == nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	group.Add(func() error {
		return m.Conf.Watch(ctx)
	}, func(err error) {
		cancel()
	})
}

func (m Module) ProvideConfig() []contract.ExportedConfig {
	return []contract.ExportedConfig{
		{
			Name: "name",
			Data: map[string]interface{}{
				"name": "skeleton",
			},
			Comment: "The name of the application",
		},
		{
			Name: "version",
			Data: map[string]interface{}{
				"version": "0.1.0",
			},
			Comment: "The version of the application",
		},
		{
			Name: "env",
			Data: map[string]interface{}{
				"env": "local",
			},
			Comment: "The environment of the application, one of production, development, staging, testing or local",
		},
		{
			Name: "http",
			Data: map[string]interface{}{
				"http": map[string]interface{}{
					"addr": ":8080",
				},
			},
			Comment: "The http address",
		},
		{
			Name: "grpc",
			Data: map[string]interface{}{
				"grpc": map[string]interface{}{
					"addr": ":9090",
				},
			},
			Comment: "The gRPC address",
		},
	}
}

type Provider interface {
	ProvideConfig() []contract.ExportedConfig
}

func (m Module) ProvideCommand(command *cobra.Command) {
	var (
		outputFile string
		style      string
	)
	exportConfigCmd := &cobra.Command{
		Use:                    "exportConfig",
		Short:                  "export default config",
		Long:                   "export a copy of default config for every added module in this application",
		BashCompletionFunction: "",
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				handler         handler
				targetFile      *os.File
				exportedConfigs []contract.ExportedConfig
				confMap         map[string]interface{}
				err             error
			)
			handler, err = getHandler(style)
			if err != nil {
				return err
			}
			_ = m.Container.GetModules().Filter(func(provider Provider) {
				exportedConfigs = append(exportedConfigs, provider.ProvideConfig()...)
			})
			if len(args) >= 1 {
				var copy = make([]contract.ExportedConfig, 0)
				for i := range exportedConfigs {
					for j := range args {
						if args[j] == exportedConfigs[i].Name {
							copy = append(copy, exportedConfigs[i])
							break
						}
					}
				}
				exportedConfigs = copy
			}
			os.MkdirAll(filepath.Dir(outputFile), 0644)
			targetFile, err = os.OpenFile(outputFile,
				handler.flags(), 0644)
			if err != nil {
				return errors.Wrap(err, "failed to open config file")
			}
			defer targetFile.Close()
			bytes, err := ioutil.ReadAll(targetFile)
			if err != nil {
				return errors.Wrap(err, "failed to read config file")
			}
			err = handler.unmarshal(bytes, &confMap)
			if err != nil {
				return errors.Wrap(err, "failed to unmarshal config file")
			}
			err = handler.write(targetFile, exportedConfigs, confMap)
			if err != nil {
				return errors.Wrap(err, "failed to write config file")
			}
			return nil
		},
	}
	exportConfigCmd.Flags().StringVarP(
		&outputFile,
		"outputFile",
		"o",
		"./config/config.yaml",
		"The output file of exported config",
	)
	exportConfigCmd.Flags().StringVarP(
		&style,
		"style",
		"s",
		"yaml",
		"The output file style",
	)
	command.AddCommand(exportConfigCmd)
}

func getHandler(style string) (handler, error) {
	switch style {
	case "json":
		return jsonHandler{}, nil
	case "yaml":
		return yamlHandler{}, nil
	default:
		return nil, fmt.Errorf("unsupported config style %s", style)
	}
}

type yamlHandler struct {
}

func (y yamlHandler) flags() int {
	return os.O_APPEND | os.O_CREATE | os.O_RDWR
}

func (y yamlHandler) unmarshal(bytes []byte, o interface{}) error {
	return yaml.Unmarshal(bytes, o)
}

func (y yamlHandler) write(file *os.File, configs []contract.ExportedConfig, confMap map[string]interface{}) error {
out:
	for _, config := range configs {
		for k := range config.Data {
			if _, ok := confMap[k]; ok {
				continue out
			}
		}
		bytes, err := yaml.Marshal(config.Data)
		if err != nil {
			return err
		}
		if config.Comment != "" {
			_, err = fmt.Fprintln(file, "# "+config.Comment)
			if err != nil {
				return err
			}
		}
		_, err = file.Write(bytes)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(file)
		if err != nil {
			return err
		}
	}
	return nil
}

type handler interface {
	flags() int
	unmarshal(bytes []byte, o interface{}) error
	write(file *os.File, configs []contract.ExportedConfig, confMap map[string]interface{}) error
}

type jsonHandler struct {
}

func (y jsonHandler) flags() int {
	return os.O_CREATE | os.O_RDWR
}

func (y jsonHandler) unmarshal(bytes []byte, o interface{}) error {
	if len(bytes) == 0 {
		return nil
	}
	return json.Unmarshal(bytes, o)
}

func (y jsonHandler) write(file *os.File, configs []contract.ExportedConfig, confMap map[string]interface{}) error {
	if confMap == nil {
		confMap = make(map[string]interface{})
	}
	for _, config := range configs {
		if _, ok := confMap[config.Name]; ok {
			continue
		}
		for k := range config.Data {
			confMap[k] = config.Data[k]
		}
	}
	file.Seek(0, 0)
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(confMap)
	return err
}