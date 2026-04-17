package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const sysCoordClientID = "system-coordinator-client"
const sysCoordSubscribeTopic = "/status/+" // subscribe to all status topics from devices, topic format: "/status/{deviceID}"
const sysCoordPublishTopic = "/system/signal"

const pathToSysCoordCert = "/etc/mosquitto/certs/laptop-client/laptop-client.crt"
const pathToSysCoordKey = "/etc/mosquitto/certs/laptop-client/laptop-client.key"

var readyDevices = make(map[string]bool)
var hasGameStarted = false // global flag

const expectedDevices = 0 // u96-client, unity-client, esp32-paddle-client, esp32-player-client

var sysCoordClient mqtt.Client

func startSystemCoordinator() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tls://%s:%d", mqttBroker, mqttPort))
	opts.SetClientID(sysCoordClientID)
	opts.OnConnect = connectHandler
	opts.OnConnectionLost = connectLostHandler

	// Loads CA certificate file
	caCert, err := os.ReadFile(pathToCA)
	if err != nil {
		panic(err)
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		panic("Error: CA file must be in PEM format")
	}
	// Loads client certificate files
	cert, err := tls.LoadX509KeyPair(pathToSysCoordCert, pathToSysCoordKey)
	if err != nil {
		panic(err)
	}
	// Instantiates a Config instance
	tlsConfig := &tls.Config{
		RootCAs:            caCertPool,
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true,
	}
	opts.SetTLSConfig(tlsConfig)

	sysCoordClient = mqtt.NewClient(opts)
	if token := sysCoordClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	devicesReadyChecker(sysCoordClient)
}

func devicesReadyChecker(client mqtt.Client) {
	// listen for READY signals from all devices on "/status/+"
	client.Subscribe(sysCoordSubscribeTopic, 0, func(c mqtt.Client, m mqtt.Message) {
		topicString := strings.Split(m.Topic(), "/") // topic format: "/status/{deviceID}"
		subscribingDeviceID := topicString[len(topicString)-1]
		payload := string(m.Payload())

		if payload == "READY" {
			// mark device as ready, duplicate READY signals from same device will not affect count
			readyDevices[subscribingDeviceID] = true
			fmt.Printf("[CHECK-IN] %s is READY. (%d/%d)\n", subscribingDeviceID, len(readyDevices), expectedDevices)
		}

		// Game can only start when all devices have checked in as READY
		if len(readyDevices) >= expectedDevices && !hasGameStarted {
			fmt.Println("[INFO] ALL DEVICES ARE INITIALISED. Starting Game...")
			token := client.Publish(sysCoordPublishTopic, 1, true, "START") // retained message, so that late subscribers can also receive the START signal
			token.Wait()
			hasGameStarted = true // set flag to prevent multiple START signals being published
			fmt.Println("[INFO] START signal broadcasted.")
		} else if hasGameStarted {
			fmt.Printf("[INFO] %s joined late. System already active.\n", subscribingDeviceID)
		}
	})
}

// Cleanup function to close system-coordinator when program is terminated.
func closeSystemCoordinator(client mqtt.Client) {
	if client == nil || !client.IsConnected() {
		return
	}

	client.Publish(sysCoordPublishTopic, 1, true, "") // clear retained START
	fmt.Println("[SYSTEM] Cleared retained messages.")
	client.Disconnect(250)
}
