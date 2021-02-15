# meterproxy

A modbus proxy daemon with MQTT reporting

## Concept

The basic concept for this daemon was to allow the electricity meter data to be captured and recorded while also being sent to the inverter. The communication between the meter and inverter is via modbus which doesn't easily allow for such setups. Given how specific this need is this may not be useful for anyone else :-)

The configuration file is used to determine the flow of data and what data from the meter is recorded.

## Hardware Setup

This runs on a RaspberryPi with 2 RS485 USB adapters, one connected to each device.

## Command Line

```cmdline
Usage of ./meterproxy:
  -cfg string
        Configuration file (default configuration.yaml) (default "configuration.yaml")
  -mode string
        Mode to start in. Used for testing/development
```

## HomeAssistant

The MQTT setup also published the discovery information for HA, allowing the data to be easily used.