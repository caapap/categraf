package spark_streaming

import (
	"log"

	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/inputs"
)

const inputName = "spark_streaming"

type SparkStreaming struct {
	config.PluginConfig
	Instances []*Instance `toml:"instances"`
}

func init() {
	inputs.Add(inputName, func() inputs.Input {
		return &SparkStreaming{}
	})
}

func (s *SparkStreaming) Clone() inputs.Input {
	return &SparkStreaming{}
}

func (s *SparkStreaming) Name() string {
	return inputName
}

func (s *SparkStreaming) GetInstances() []inputs.Instance {
	ret := make([]inputs.Instance, len(s.Instances))
	for i := 0; i < len(s.Instances); i++ {
		ret[i] = s.Instances[i]
	}
	return ret
}

func (s *SparkStreaming) Drop() {
	for _, inst := range s.Instances {
		if inst != nil {
			inst.Drop()
		}
	}
	log.Println("I! [spark_streaming] Plugin stopped")
}

