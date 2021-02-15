package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/goburrow/serial"
	"github.com/tbrandon/mbserver"
)

type request struct {
	conn  io.ReadWriteCloser
	frame *mbserver.RTUFrame
}

const rtuMinSz = 8

var requestChan = make(chan *request)

func startServer(quitChannel chan bool) error {
	rtuConfig := serial.Config{
		Address:  appConfig.Server.Devicename,
		BaudRate: appConfig.Server.Baudrate,
		DataBits: 8,
		StopBits: 1,
		Parity:   appConfig.Server.Parity,
		Timeout:  1 * time.Second,
	}

	go processRequest()

	port, err := serial.Open(&rtuConfig)
	if err != nil {
		return fmt.Errorf("failed to open %s: %v", rtuConfig.Address, err)
	}

	go acceptSerialRequests(port, quitChannel)
	log.Printf("Server: Started listening on %s", appConfig.Server.Devicename)
	return nil
}

type frameBuffer struct {
	buf bytes.Buffer
}

func acceptSerialRequests(port serial.Port, quitChannel chan bool) {
	fb := frameBuffer{}
	tmpBuf := make([]byte, 16)

	for {
		// Read bytes into the buffer until we have enough to start
		// looking for frames...
		b, err := port.Read(tmpBuf)
		if err != nil {
			if err == serial.ErrTimeout {
				continue
			}
			log.Printf("Server: %v", err)
			if err == io.EOF {
				log.Printf("Exiting listen loop.")
				quitChannel <- true
				break
			}
			continue
		}
		if b == 0 {
			continue
		}
		fb.buf.Write(tmpBuf[:b])

		for fb.buf.Len() >= rtuMinSz {
			frame, err := mbserver.NewRTUFrame(fb.buf.Next(rtuMinSz))
			if err != nil {
				log.Printf("bad serial frame error %v\n", err)
				log.Print(fb.buf.Bytes())
				fb.buf.Reset()
				continue
			}
			requestChan <- &request{port, frame}
		}
	}
}

func processRequest() {
	for {
		req := <-requestChan
		//		log.Printf("Server: RX: %02X %02X %s", req.frame.Address, req.frame.Function, hex.EncodeToString(req.frame.Data))

		var (
			regA  *registerAccess
			bytes []byte
			err   modbusError
			out   []byte
		)
		switch req.frame.Function {
		case 1, 2, 3, 4:
			regA, err = getRegisterAccess(req.frame.Address, req.frame.Function)
		case 6:
			regA, err = getRegisterAccess(req.frame.Address, 3)
		}
		if err == modbusSuccess {
			register := int(binary.BigEndian.Uint16(req.frame.Data[0:2]))
			numRegs := int(binary.BigEndian.Uint16(req.frame.Data[2:4]))
			dLen := len(req.frame.Data)

			switch req.frame.Function {
			case 3, 4:
				bytes, err = regA.Read(register, numRegs)
			// Write is untested
			case 6:
				err = regA.Write(register, numRegs, req.frame.Data[5:dLen])
			}
		}

		if err == modbusSuccess {
			out = []byte{req.frame.Address, req.frame.Function}
			out = append(out, bytes...)
		} else {
			fn := req.frame.Function | 0x80
			out = []byte{req.frame.Address, fn}
			out = append(out, err.code)
		}
		crc := modbusCRC(out)

		//		log.Printf("Server: TX: %02X %02X %s", out[0], out[1], hex.EncodeToString(out[2:]))

		oLen := len(out)
		out = append(out, []byte{0, 0}...)
		binary.LittleEndian.PutUint16(out[oLen:oLen+2], crc)

		_, wErr := req.conn.Write(out)
		if wErr != nil {
			log.Print(wErr)
			break
		}
	}
}
