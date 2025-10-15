package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const DEV_PATH = "/sys/bus/usb/devices"

var (
	devRe            regexp.Regexp = *regexp.MustCompile(`[0-9]-[0-9]`)
	usbSerialDevices               = make(map[string]bool, 10)
	usbSerialRe                    = regexp.MustCompile(`(?i)usb.*serial|serial.*usb`)
)

func findUSBSerialDevices() {
	files, err := filepath.Glob(filepath.Join(DEV_PATH, "usb*"))
	if err != nil {
		log.Fatalf("No USB devices available: %v", err)
	}

	for _, poss := range files {
		productFnGlob := filepath.Join(poss, "*", "product")
		files, err := filepath.Glob(productFnGlob)
		if err != nil || len(files) == 0 {
			continue
		}
		for _, prodFn := range files {
			product := readFile(prodFn)
			if !usbSerialRe.MatchString(product) {
				continue
			}
			ttyGlob := filepath.Join(filepath.Dir(prodFn), "*:*", "tty*")
			//fmt.Printf("Looking for TTY's: %s\n", ttyGlob)
			ttys, err := filepath.Glob(ttyGlob)
			if err != nil || len(ttys) == 0 {
				continue
			}
			found := filepath.Base(ttys[0])
			_, ck := usbSerialDevices[found]
			if !ck {
				usbSerialDevices[found] = false
			}
		}
	}
}

func readFile(fn string) (rStr string) {
	fh, err := os.Open(fn)
	if err != nil {
		return
	}
	defer fh.Close()
	rBytes, err := ioutil.ReadAll(fh)
	if err != nil {
		return
	}
	rStr = strings.TrimSuffix(string(rBytes), "\n")
	return
}
