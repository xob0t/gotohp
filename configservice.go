package main

import (
	"fmt"
	"log"
	"os"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

type GlobalSettings struct {
	Credentials []string `json:"credentials" koanf:"credentials"`
	Selected    string   `json:"selected" koanf:"selected"`
	Proxy       string   `json:"proxy" koanf:"proxy"`
}

type ConfigService struct{}

var GlobalSettingsConfig GlobalSettings
var ActionRunning bool = false
var GlobalSettingsPath string = "./config.yaml"
var DefaultConfig = GlobalSettings{}

func Exists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

func (g *ConfigService) SetPropxy(proxy string) {
	GlobalSettingsConfig.Proxy = proxy
	SaveGlobalConfig()
}

func (g *ConfigService) SetSelected(selected string) {
	GlobalSettingsConfig.Selected = selected
	SaveGlobalConfig()
}

func (g *ConfigService) GetConfig(_ struct{}) GlobalSettings {
	configExists := Exists(GlobalSettingsPath)
	if !configExists {
		fmt.Println("Created a new user settings config")
		GlobalSettingsConfig = DefaultConfig
	}
	file, _ := os.ReadFile(GlobalSettingsPath)
	if len(file) == 0 {
		fmt.Println("config file is empty")
		GlobalSettingsConfig = DefaultConfig
	} else {
		GlobalSettingsConfig, _ = parseGlobalConfig()
	}

	log.Println("Config", GlobalSettingsConfig)
	return GlobalSettingsConfig
}

func SaveGlobalConfig() error {
	k := koanf.New(".")

	err := k.Load(structs.Provider(GlobalSettingsConfig, "koanf"), nil)
	if err != nil {
		fmt.Println(err)
		return err
	}
	b, err := k.Marshal(yaml.Parser())
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = os.WriteFile(GlobalSettingsPath, b, os.ModePerm)

	if err != nil {
		fmt.Println(err)
		return err
	}

	return nil
}

func parseGlobalConfig() (GlobalSettings, error) {
	var c GlobalSettings
	var k = koanf.New(".")
	if err := k.Load(file.Provider(GlobalSettingsPath), yaml.Parser()); err != nil {
		log.Printf("error loading global config: %v", err)
		return DefaultConfig, err
	}
	err := k.Unmarshal("", &c)
	if err != nil {
		log.Printf("error Unmarshaling global config: %v", err)
		return DefaultConfig, err
	}

	return c, nil
}
