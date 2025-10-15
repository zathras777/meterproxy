package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"sync"
)

type registerData [256]uint16

type register struct {
	data [256]uint16
	rw   sync.RWMutex
}

type registerReader func(*register, int, int) ([]byte, modbusError)
type registerWriter func(*register, int, int, []byte) modbusError

type registerAccess struct {
	reg    *register
	reader registerReader
	writer registerWriter
}

var standardRegisters = map[byte]registerAccess{
	3: {reader: readRegisters, writer: writeRegisters},
	4: {reader: readRegisters, writer: writeRegisters},
}

var devices = make(map[byte]map[byte]*registerAccess)

func makeRegisterAccess(reader registerReader, writer registerWriter) *registerAccess {
	reg := register{}
	ra := registerAccess{reg: &reg, reader: reader, writer: writer}
	return &ra
}

func addStandardDevice(deviceNum byte) error {
	_, ck := devices[deviceNum]
	if ck {
		return fmt.Errorf("device %d already registered?", deviceNum)
	}
	devices[deviceNum] = make(map[byte]*registerAccess)
	devices[deviceNum][3] = makeRegisterAccess(readRegisters, writeRegisters)
	devices[deviceNum][4] = makeRegisterAccess(readRegisters, writeRegisters)
	log.Printf("Server: Added Standard device #%d", deviceNum)
	return nil
}

func getRegisterAccess(deviceNum, function byte) (*registerAccess, modbusError) {
	deviceRegisters, ck := devices[deviceNum]
	if !ck {
		//		log.Printf("Request for unregistered device #%d", deviceNum)
		return nil, unknownDevice
	}
	regA, ck := deviceRegisters[function]
	if !ck {
		log.Printf("Request for unregistered function #%d on device #%d", function, deviceNum)
		return nil, illegalFunction
	}
	return regA, modbusSuccess
}

func (ra *registerAccess) Read(regStart, numReg int) ([]byte, modbusError) {
	if ra.reader == nil {
		return []byte{}, illegalFunction //fmt.Errorf("No reader function available")
	}
	return ra.reader(ra.reg, regStart, numReg)
}

func (ra *registerAccess) Write(regStart, numReg int, bytes []byte) modbusError {
	if ra.writer == nil {
		return illegalFunction //fmt.Errorf("No writer function available")
	}
	return ra.writer(ra.reg, regStart, numReg, bytes)
}

func readRegisters(reg *register, regStart, numReg int) ([]byte, modbusError) {
	regEnd := regStart + numReg
	bytes := make([]byte, numReg*2+1)
	bytes[0] = byte(numReg * 2)

	idx := 1
	reg.rw.RLock()
	for n := regStart; n < regEnd; n++ {
		binary.BigEndian.PutUint16(bytes[idx:idx+2], reg.data[n])
		idx += 2
	}
	reg.rw.RUnlock()
	return bytes, modbusSuccess
}

func writeRegisters(reg *register, regStart, numReg int, bytes []byte) modbusError {
	regEnd := regStart + numReg

	idx := 0
	reg.rw.Lock()
	for n := regStart; n < regEnd; n++ {
		val := uint16(bytes[idx])<<8 + uint16(bytes[idx+1])
		reg.data[n] = val
		idx += 2
	}
	reg.rw.Unlock()
	return modbusSuccess
}
