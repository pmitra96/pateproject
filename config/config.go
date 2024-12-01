// config.go
package config

import (
	"io/ioutil"
	"pateproject/entity"
	"pateproject/logger"

	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

// ReadConfig reads the configuration from the YAML file
func ReadConfig(filePath string) (*entity.Config, error) {
	var config entity.Config

	// Read the YAML file content
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		logger.Error("unable to read file", zap.Error(err))
		return nil, err
	}

	// Unmarshal the YAML data into the Config struct
	if err := yaml.Unmarshal(data, &config); err != nil {
		logger.Error("unable to unmarshal YAML", zap.Error(err))
		return nil, err
	}

	return &config, nil
}
