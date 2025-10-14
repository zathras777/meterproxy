package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type publishCall struct {
	topic    string
	qos      byte
	retained bool
	payload  interface{}
}

type fakeClient struct {
	connected  bool
	connectErr error
	publishes  []publishCall
}

func (f *fakeClient) IsConnected() bool {
	return f.connected
}

func (f *fakeClient) IsConnectionOpen() bool {
	return f.connected
}

func (f *fakeClient) Connect() mqtt.Token {
	if f.connectErr == nil {
		f.connected = true
	}
	return newFakeToken(f.connectErr)
}

func (f *fakeClient) Disconnect(quiesce uint) {
	f.connected = false
}

func (f *fakeClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	f.publishes = append(f.publishes, publishCall{topic: topic, qos: qos, retained: retained, payload: payload})
	return newFakeToken(nil)
}

func (f *fakeClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return newFakeToken(nil)
}

func (f *fakeClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	return newFakeToken(nil)
}

func (f *fakeClient) Unsubscribe(topics ...string) mqtt.Token {
	return newFakeToken(nil)
}

func (f *fakeClient) AddRoute(topic string, callback mqtt.MessageHandler) {}

func (f *fakeClient) OptionsReader() mqtt.ClientOptionsReader {
	return mqtt.ClientOptionsReader{}
}

type fakeToken struct {
	err  error
	done chan struct{}
}

func newFakeToken(err error) *fakeToken {
	done := make(chan struct{})
	close(done)
	return &fakeToken{err: err, done: done}
}

func (t *fakeToken) Wait() bool {
	return true
}

func (t *fakeToken) WaitTimeout(_ time.Duration) bool {
	return true
}

func (t *fakeToken) Done() <-chan struct{} {
	return t.done
}

func (t *fakeToken) Error() error {
	return t.err
}

func setupTestEnvironment(t *testing.T) *registerAccess {
	t.Helper()

	devices = make(map[byte]map[byte]*registerAccess)

	appConfig = configData{
		Name: "TestDevice",
		MQTT: mqttData{
			Host:                "localhost",
			Port:                1883,
			QoS:                 1,
			TopicPrefix:         "prefix",
			HassdiscoveryPrefix: "ha",
		},
	}
	appConfig.Source.DeviceID = 1

	field := recordField{
		Name:  "Power",
		Idx:   0,
		Units: "W",
		uid:   "power",
	}
	field.topic = fmt.Sprintf("%s/%s/%s/state", appConfig.MQTT.TopicPrefix, appConfig.Name, field.uid)
	appConfig.Source.Fields = []recordField{field}

	if err := addStandardDevice(appConfig.Source.DeviceID); err != nil {
		t.Fatalf("addStandardDevice: %v", err)
	}
	regA, modErr := getRegisterAccess(appConfig.Source.DeviceID, 4)
	if modErr != modbusSuccess {
		t.Fatalf("getRegisterAccess: %v", modErr)
	}
	return regA
}

func writeFloatToRegister(t *testing.T, regA *registerAccess, value float32) {
	t.Helper()
	raw := make([]byte, 4)
	binary.BigEndian.PutUint32(raw, math.Float32bits(value))
	if wErr := regA.Write(appConfig.Source.Fields[0].Idx, 2, raw); wErr != modbusSuccess {
		t.Fatalf("register write failed: %v", wErr)
	}
}

func TestExecutePublishesWhenConnected(t *testing.T) {
	regA := setupTestEnvironment(t)
	writeFloatToRegister(t, regA, 12.34)

	client := &fakeClient{connected: true}
	mqttClient = client
	t.Cleanup(func() { mqttClient = nil })

	if err := execute(); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	if len(client.publishes) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(client.publishes))
	}

	p := client.publishes[0]
	if p.topic != appConfig.Source.Fields[0].topic {
		t.Fatalf("unexpected topic: got %s want %s", p.topic, appConfig.Source.Fields[0].topic)
	}
	payload, ok := p.payload.(string)
	if !ok {
		t.Fatalf("expected string payload, got %T", p.payload)
	}
	expectedPayload := fmt.Sprintf("%.02f", float32(12.34))
	if payload != expectedPayload {
		t.Fatalf("unexpected payload: got %s want %s", payload, expectedPayload)
	}
}

func TestExecuteSkipsWhenClientDisconnected(t *testing.T) {
	regA := setupTestEnvironment(t)
	writeFloatToRegister(t, regA, 45.67)

	client := &fakeClient{connected: false}
	mqttClient = client
	t.Cleanup(func() { mqttClient = nil })

	if err := execute(); err != nil {
		t.Fatalf("execute returned error: %v", err)
	}

	if len(client.publishes) != 0 {
		t.Fatalf("expected no publishes, got %d", len(client.publishes))
	}
}

func TestRegisterHAPublishesOnlyWhenConnected(t *testing.T) {
	setupTestEnvironment(t)

	client := &fakeClient{connected: false}
	mqttClient = client
	t.Cleanup(func() { mqttClient = nil })

	registerHA()
	if len(client.publishes) != 0 {
		t.Fatalf("expected no publishes when disconnected, got %d", len(client.publishes))
	}

	client.connected = true
	registerHA()
	if len(client.publishes) != len(appConfig.Source.Fields) {
		t.Fatalf("expected %d publishes, got %d", len(appConfig.Source.Fields), len(client.publishes))
	}

	p := client.publishes[0]
	expectedTopic := fmt.Sprintf("%s/sensor/%s/%d/config", appConfig.MQTT.HassdiscoveryPrefix,
		appConfig.Name, appConfig.Source.Fields[0].Idx)
	if p.topic != expectedTopic {
		t.Fatalf("unexpected HA topic: got %s want %s", p.topic, expectedTopic)
	}
	payloadBytes, ok := p.payload.([]byte)
	if !ok {
		t.Fatalf("expected []byte payload, got %T", p.payload)
	}
	payload := string(payloadBytes)
	for _, want := range []string{`"name":"TestDevice Power"`, `"unique_id":"power"`, `"state_topic":"prefix/TestDevice/power/state"`} {
		if !strings.Contains(payload, want) {
			t.Fatalf("expected payload to contain %s, got %s", want, payload)
		}
	}
}
