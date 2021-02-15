package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/zathras777/modbusdev"
)

var (
	val        modbusdev.Value
	mqttClient mqtt.Client
)

// Execute Execute the stored query using supplied map of values
func execute() (err error) {
	regA, err := getRegisterAccess(appConfig.Source.DeviceID, 4)
	if err != modbusSuccess {
		log.Printf("Error getting registerAccess: %s", err)
		return
	}

	for _, fld := range appConfig.Source.Fields {
		data, err := regA.Read(fld.Idx, 2)
		if err != modbusSuccess {
			log.Printf("Unable to access index %d: %s", fld.Idx, err)
			continue
		}
		val.FormatBytes("ieee32", data[1:])
		token := mqttClient.Publish(fld.topic, appConfig.MQTT.QoS, true, fmt.Sprintf("%.02f", val.Ieee32))
		token.Wait()
	}
	return nil
}

func startRecording() {
	mqOpts := mqtt.NewClientOptions()
	mqOpts.AddBroker(fmt.Sprintf("tcp://%s:%d", appConfig.MQTT.Host, appConfig.MQTT.Port))

	mqttClient = mqtt.NewClient(mqOpts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Printf("Unable to connect to the MQTT server on bob: %v\n", token.Error())
		mqttClient = nil
	}

	registerHA()
	for {
		if err := execute(); err != nil {
			break
		}
		time.Sleep(time.Second)
	}
}

func registerHA() {
	type hassAdvert struct {
		Name              string `json:"name"`
		UniqueID          string `json:"unique_id"`
		Icon              string `json:"icon,omitempty"`
		StateTopic        string `json:"state_topic"`
		UnitOfMeasurement string `json:"unit_of_measurement,omitempty"`
	}
	for _, fld := range appConfig.Source.Fields {
		haData := hassAdvert{
			Name:              fmt.Sprintf("%s %s", appConfig.Name, fld.Name),
			StateTopic:        fld.topic,
			UniqueID:          fld.uid,
			UnitOfMeasurement: fld.Units}
		switch fld.Units {
		case "W", "kWh":
			haData.Icon = "hass:flash"
		}
		jsonBytes, err := json.Marshal(haData)
		if err != nil {
			log.Printf("Unable to encode HA json: %s", err)
			continue
		}
		mqttClient.Publish(fmt.Sprintf("%s/sensor/%s/%d/config", appConfig.MQTT.HassdiscoveryPrefix,
			appConfig.Name, fld.Idx), appConfig.MQTT.QoS, true, jsonBytes)
	}
}
