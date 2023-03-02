package crapdav


import (
	"os"
	"fmt"
	"gopkg.in/ini.v1"
)

type Config struct {
	Calendar string
}


func ParseConfig(configFile string) (error, Config) {
	if configFile == "" {
		configFile = os.ExpandEnv("$HOME/.crapdav.ini")
	}
	cfg, err := ini.Load(configFile)
	if err != nil {
		return err, Config{}
	}

	calendar := os.ExpandEnv(cfg.Section("").Key("calendar").String())
	if calendar == "" {
		return fmt.Errorf("calendar not defined in the config"), Config{}
	}
	return nil, Config{
		Calendar: calendar,
	}
}
