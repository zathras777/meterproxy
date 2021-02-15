package main

import (
	"fmt"
	"io/ioutil"
	"strings"

	"gopkg.in/yaml.v2"
)

type regRange struct {
	Start, Finish int
	Delay         int
}

type remoteDevice struct {
	ID     byte
	Ranges []regRange
}

type rtuData struct {
	Devicename string
	Baudrate   int
	Parity     string
	Devices    []remoteDevice
}

type mqttData struct {
	Host                string
	Port                uint
	QoS                 byte
	TopicPrefix         string `yaml:"topic_prefix"`
	HassdiscoveryPrefix string `yaml:"hassdiscovery_prefix"`
}

type recordField struct {
	Name  string
	Idx   int
	Units string
	uid   string
	topic string
}

type configData struct {
	Name   string
	Server rtuData
	MQTT   mqttData
	Source struct {
		DeviceID byte `yaml:"device_id"`
		Fields   []recordField
	}
	Clients []rtuData
}

var appConfig configData

func parseConfiguration(cfgFn string) (err error) {
	appConfig.MQTT.QoS = 0

	cfgData, err := ioutil.ReadFile(cfgFn)
	if err != nil {
		return
	}

	err = yaml.Unmarshal(cfgData, &appConfig)
	if err != nil {
		fmt.Println(err)
		return
	}

	var newFields []recordField
	for _, fld := range appConfig.Source.Fields {
		fld.uid = strings.ReplaceAll(strings.ToLower(fld.Name), " ", "_")
		fld.topic = fmt.Sprintf("%s/%s/%s/state", appConfig.MQTT.TopicPrefix, appConfig.Name, fld.uid)
		newFields = append(newFields, fld)
	}
	appConfig.Source.Fields = newFields
	fmt.Println(appConfig)
	return
}
