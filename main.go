package main

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/goburrow/serial"
)

func main() {
	var mode string
	var cfgFn string

	flag.StringVar(&mode, "mode", "", "Mode to start in. Used for testing/development")
	flag.StringVar(&cfgFn, "cfg", "configuration.yaml", "Configuration file (default configuration.yaml)")

	flag.Parse()

	fmt.Printf("Meter Proxy. Reading configuration from %s\n", cfgFn)

	findUSBSerialDevices()
	if len(usbSerialDevices) == 0 {
		fmt.Println("Unable to find any suitable USB devices? Exiting...")
		os.Exit(1)
	}

	logwriter, e := syslog.New(syslog.LOG_DEBUG|syslog.LOG_DAEMON, "meterproxy")
	if e == nil {
		log.SetOutput(logwriter)
	}

	if err := parseConfiguration(cfgFn); err != nil {
		log.Fatal(err)
	}

	quitChannel := make(chan bool)
	// Start client to collect data before we start serving responses.
	if mode == "" || mode == "client" {
		for _, client := range appConfig.Clients {
			err := startClient(client)
			if err != nil {
				fmt.Println(err)
				log.Fatal(err)
			}
		}
		time.Sleep(time.Second)
	}

	if mode == "" || mode == "server" {
		if err := startServer(quitChannel); err != nil {
			log.Fatal(err)
		}
		addStandardDevice(1)
	}

	if mode == "" {
		go startRecording()
	} else {
		log.Print("Database recording not being started due operating mode")
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		log.Print("Quit signal received, exiting...")
		quitChannel <- true
	}()

	if mode == "client" {
		fmt.Println("Started as client. Will run until CTRL+C used.")
	}

	<-quitChannel
}

func testWrite() {
	port, err := serial.Open(&serial.Config{Address: "/dev/ttyUSB1", BaudRate: 9600, Parity: "N"})
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	_, err = port.Write([]byte{1, 3, 0, 14, 0, 1, 229, 201})
	if err != nil {
		log.Fatal(err)
	}
}
