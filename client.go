package main

/* Each client represents an endpoint with one or more devices attached.
 * Each device is configured with the modbus ID and lists of register ranges to be
 * read and stored by the server.
 * One limitation is that the server can only store a single set of device data, so every
 * configured device ID MUST be unique across all configured clients.
 */

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/goburrow/modbus"
)

type deviceAction struct {
	opType         int
	startRegister  uint16
	finishRegister uint16
	numRegs        uint16
	errors         int
	delay          time.Duration
}

type device struct {
	id      byte
	client  modbus.Client
	actions []*deviceAction
}

type deviceBus struct {
	devices []device
}

var clientDevices []device
var deviceBusses []deviceBus
var maxErrors int = 10
var defaultDelay time.Duration = 500

func getRegisterType(num int) (typ int, reg uint16) {
	sVal := fmt.Sprintf("%d", num)
	typ, err := strconv.Atoi(string(sVal[0]))
	if err != nil {
		log.Print(err)
	}
	regi, err := strconv.Atoi(string(sVal[1:]))
	if err != nil {
		log.Print(err)
	}
	reg = uint16(regi) - 1
	return
}

func startClient(cfg rtuData) error {
	bus := deviceBus{}
	for _, dev := range cfg.Devices {
		addStandardDevice(dev.ID)

		handler := modbus.NewRTUClientHandler(cfg.Devicename)
		handler.BaudRate = cfg.Baudrate
		handler.DataBits = 8
		handler.Parity = cfg.Parity
		handler.StopBits = 1
		handler.SlaveId = dev.ID
		handler.Timeout = 1 * time.Second

		err := handler.Connect()
		if err != nil {
			log.Printf("Unable to connect: %s\n", err)
			return err
		}

		//		defer handler.Close()
		client := modbus.NewClient(handler)
		cDev := device{id: dev.ID, client: client}
		for _, rng := range dev.Ranges {
			da, err := deviceActionFromConfig(rng)
			if err != nil {
				log.Print(err)
				continue
			}
			cDev.actions = append(cDev.actions, da)
		}
		if len(cDev.actions) == 0 {
			log.Printf("No valid register ranges found for device %d on %s", dev.ID, cfg.Devicename)
			continue
		}
		bus.devices = append(bus.devices, cDev)
	}
	if len(bus.devices) == 0 {
		log.Printf("No devices configured for device bus %s", cfg.Devicename)
		return nil
	}
	deviceBusses = append(deviceBusses, bus)
	//	fmt.Println("Starting collector for client data")
	go bus.collect()
	return nil
}

func deviceActionFromConfig(rng regRange) (*deviceAction, error) {
	sType, startReg := getRegisterType(rng.Start)
	fType, finishReg := getRegisterType(rng.Finish)
	if sType != fType {
		return nil, fmt.Errorf("Range types do not match. %d vs %d", sType, fType)
	}
	if finishReg < startReg {
		return nil, fmt.Errorf("Finish register was lower than start register??? %d vs %d", startReg, finishReg)
	}
	delay := defaultDelay
	if rng.Delay > 0 {
		delay = time.Duration(rng.Delay)
	}
	return &deviceAction{opType: sType, startRegister: startReg, finishRegister: finishReg, numRegs: finishReg - startReg, delay: delay}, nil
}

func (bus deviceBus) collect() {
OuterLoop:
	for {
		for _, dev := range bus.devices {
			for _, act := range dev.actions {
				if act.errors >= maxErrors {
					log.Printf("Device %d: Skipping action %v due excessive errors", dev.id, act)
					break OuterLoop
				}
				var (
					results []byte
					err     error
					mErr    modbusError
					regA    *registerAccess
				)
				switch act.opType {
				case 3:
					//log.Printf("ReadInputRegisters(%d, %d)", act.startRegister, act.numRegs)
					results, err = dev.client.ReadInputRegisters(act.startRegister, act.numRegs)
					regA, mErr = getRegisterAccess(dev.id, 4)
				case 4:
					//log.Printf("ReadHoldingRegisters(%d, %d)", act.startRegister, act.numRegs)
					results, err = dev.client.ReadHoldingRegisters(act.startRegister, act.numRegs)
					regA, mErr = getRegisterAccess(dev.id, 3)
				}
				if err != nil {
					act.errors++
					log.Printf("Device %d: %v failed: %c", dev.id, act, err)
					continue
				}
				if len(results) == 0 {
					continue
				}
				//log.Printf("Results: %d bytes, % x", len(results), results)

				if mErr != modbusSuccess {
					log.Print(err)
					continue
				}
				mErr = regA.Write(int(act.startRegister), int(act.numRegs), results)
				if mErr != modbusSuccess {
					log.Printf("Unable to write data to registers: %s", err)
				}
				time.Sleep(act.delay * time.Millisecond)
			}
		}
	}
}

func opString(op int) string {
	switch op {
	case 3:
		return "ReadInputRegisters"
	case 4:
		return "ReadHoldingRegisters"
	default:
		return fmt.Sprintf("OpCode %d", op)
	}
}

func (act deviceAction) String() string {
	return fmt.Sprintf("%s from %d to %d", opString(act.opType), act.startRegister, act.finishRegister)
}
