package ollama

import (
	"flashcat.cloud/categraf/config"
	"flashcat.cloud/categraf/inputs"
)

const inputName = "ollama"

// Ollama 插件主结构
type Ollama struct {
	config.PluginConfig
	Instances []*Instance `toml:"instances"`
}

func init() {
	inputs.Add(inputName, func() inputs.Input {
		return &Ollama{}
	})
}

func (o *Ollama) Clone() inputs.Input {
	return &Ollama{}
}

func (o *Ollama) Name() string {
	return inputName
}

func (o *Ollama) GetInstances() []inputs.Instance {
	ret := make([]inputs.Instance, len(o.Instances))
	for i := 0; i < len(o.Instances); i++ {
		ret[i] = o.Instances[i]
	}
	return ret
}

