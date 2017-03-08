package main

import (
	"github.com/TIBCOSoftware/flogo-lib/engine"
)

const configFileName string = "config.json"
const triggersConfigFileName string = "triggers.json"

// can be used to compile in config file
const configJSON string = ``

// can be used to compile in triggers config file
const triggersConfigJSON string = ``

// GetEngineConfig gets the engine configuration
func GetEngineConfig() *engine.Config {

	config := engine.LoadConfigFromFile(configFileName)
	//config := engine.LoadConfigFromJSON(configJSON)

	if config == nil {
		config = engine.DefaultConfig()
		log.Warningf("Configuration file '%s' not found, using defaults", configFileName)
	}

	return config
}

// GetTriggersConfig gets the triggers configuration
func GetTriggersConfig() *engine.TriggersConfig {

	config := engine.LoadTriggersConfigFromFile(triggersConfigFileName)
	//config := engine.LoadTriggersConfigFromJSON(triggersConfigJSON)

	if config == nil {
		config = engine.DefaultTriggersConfig()
		log.Warningf("Configuration file '%s' not found, using defaults", triggersConfigFileName)
	}

	return config
}
