package main

import (
	"bufio"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type Star struct {
	Conf Config `yaml:"star"`
}

type Config struct {
	Version        string            `yaml:"version"`
	NodeId         string            `yaml:"nodeid"`
	RTopic         string            `yaml:"rtopic"`
	STopic         string            `yaml:"stopic"`
	ErrTopic       string            `yaml:"errtopic"`
	Name           string            `yaml:"name"`
	Flusher        string            `yaml:"flusher"`
	Retention      string            `yaml:"retention"`
	Healthcheck    map[string]string `yaml:"healthcheck"`
	InstrumentConf map[string]string `yaml:"instrument"`
	Labels         map[string]string `yaml:"labels"`
}

func ConfigFile(n ...string) (*Config, string, error) {
	path := "config.yml"
	if len(n) > 0 {
		path = n[0]
	}

	yamlFile, err := ioutil.ReadFile(path)
	check(err)

	var conf Star
	err = yaml.Unmarshal(yamlFile, &conf)
	check(err)

	return &conf.Conf, path, nil
}

func (c *Config) SaveFile(path string) error {
	data, err := yaml.Marshal(&Star{Conf: *c})
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	} else {
		datawriter := bufio.NewWriter(file)
		datawriter.WriteString(string(data))
		datawriter.Flush()
	}
	file.Close()
	return nil
}

func (c *Config) UpdateId(newId string) {
	c.NodeId = newId
}
func (c *Config) UpdateSyncTopic(newTopic string) {
	c.RTopic = newTopic
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
