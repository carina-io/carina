package configuration

import (
	"fmt"
	"testing"

	"reflect"

	"github.com/carina-io/carina/utils/log"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

func TestUnmarshalWithDecoderOptions(t *testing.T) {
	var v = viper.New()
	v.AddConfigPath("/etc/carina/")
	v.SetConfigName("config")
	v.SetConfigType("json")
	err := v.ReadInConfig()
	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("Fatal error config file: %w \n", err))
	}
	opt := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		// Custom Decode Hook Function
		func(rf reflect.Kind, rt reflect.Kind, data interface{}) (interface{}, error) {
			if rf != reflect.Map || rt != reflect.Struct {
				return data, nil
			}
			mapstructure.Decode(data.(map[string]interface{}), &diskConfig)
			mapstructure.Decode(data.(map[string]interface{})["diskselector"], &diskConfig.DiskSelectors)
			return data, err
		},
	))

	err = v.Unmarshal(&diskConfig, opt)
	if err != nil {
		t.Fatalf("unable to decode into struct, %v", err)
	}
	log.Info(diskConfig)

}
