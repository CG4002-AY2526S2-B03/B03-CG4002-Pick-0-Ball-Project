// Initiates a network-sniffer-client to sniff the broker and calculate network metrics.
package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const snifferClientID = "network-sniffer-client"
const snifferSubscribeTopic = "#" // subscribe to all topics

const pathToCert = "/etc/mosquitto/certs/network-sniffer-client/network-sniffer-client.crt"
const pathToKey = "/etc/mosquitto/certs/network-sniffer-client/network-sniffer-client.key"
const pathToCA = "/etc/mosquitto/certs/ca.crt"

// A Network Sniffer receives all messages from the MQTT broker and calculates network metrics for each topic.
// All messages are logged in the terminal.
type Sniffer struct {
	mutex              sync.Mutex
	initializedTime    time.Time
	topicsNetworkStats map[string]*TopicStats
	topicChannels      map[string]chan mqtt.Message // maps topic to messageChan
	logChan            chan mqtt.Message            // channel for logging all messages
	esp32Active        bool                         // TODO: remove after initial demo
}

// TopicStats holds all network metrics for a particular MQTT topic.
// Updated by the TopicWorker goroutine for the topic
// Read by the displayNetworkStats goroutine every X seconds to display metrics.
// Data race handled by Sniffer's mutex as there is one writer and one reader per topic.
type TopicStats struct {
	startTime       time.Time
	lastSequenceNum int
	msgCounter      int
	byteCounter     int
	lastMsgCounter  int
	lastByteCounter int
	lastUpdateTime  time.Time

	totalMsgDrop         int
	retranmissionCounter int
	latencyStats         []int64
}

var connectHandler mqtt.OnConnectHandler = func(client mqtt.Client) {
	fmt.Println("Connected network-sniffer-client")
}

var connectLostHandler mqtt.ConnectionLostHandler = func(client mqtt.Client, err error) {
	fmt.Printf("Connect lost network-sniffer-client: %v", err)

}

func startNetworkSniffer() *Sniffer {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tls://%s:%d", mqttBroker, mqttPort))
	opts.SetClientID(snifferClientID)
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
	cert, err := tls.LoadX509KeyPair(pathToCert, pathToKey)
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

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	}

	sniffer := &Sniffer{
		initializedTime:    time.Now(),
		topicsNetworkStats: make(map[string]*TopicStats),
		topicChannels:      make(map[string]chan mqtt.Message),
		logChan:            make(chan mqtt.Message, 1000), // buffered channel for logging messages

	}
	handleTopicWorkers(client, sniffer)

	// Setup terminal for displaying network metrics and logs
	fmt.Print("\033[2J")
	fmt.Print("\033[1;23r")
	fmt.Print("\033[23;1H")

	go displayNetworkStats(sniffer) // new goroutine to handle displaying network metrics
	go sniffer.logAllMessages()     // new goroutine to log all messages

	return sniffer
}

// Handler for sniffer to subscribe to all topics.
// For each topic, start a TopicWorker and route messages to its respective channels based on msg's topic
func handleTopicWorkers(client mqtt.Client, sniffer *Sniffer) {
	client.Subscribe(snifferSubscribeTopic, 0, func(client mqtt.Client, msg mqtt.Message) { // QoS = 0
		topic := msg.Topic()

		if topic == "/paddle" || topic == "/playerPosition" { // TODO: remove if needed, these topics from hardware does flood the sniffer
			return
		}

		sniffer.mutex.Lock() // lock to safely check and create topic channel if it doesn't exist
		ch, exists := sniffer.topicChannels[topic]

		if !exists {
			ch = make(chan mqtt.Message, 100) // create a new channel for this topic
			sniffer.topicChannels[topic] = ch
			go sniffer.startTopicWorker(topic, ch) // start a worker for this topic
		}
		sniffer.mutex.Unlock()
		ch <- msg // send message to the topic's channel for processing by the worker
	})
}

// TopicWorker goroutine for each topic receives messages from its topicChannel and updates network metrics in Sniffer.topicsNetworkStats.
func (sniffer *Sniffer) startTopicWorker(topic string, ch chan mqtt.Message) {
	sniffer.mutex.Lock()
	// Check if TopicStats exist for this topic.
	if _, exists := sniffer.topicsNetworkStats[topic]; !exists {
		sniffer.topicsNetworkStats[topic] = &TopicStats{
			startTime:      time.Now(), // initialise topic stats with start time as now
			lastUpdateTime: time.Now(),
		} // all other fields are initialised to 0
	}
	stats := sniffer.topicsNetworkStats[topic] // gets a pointer to the topic's stats struct, no need for mutex!
	sniffer.mutex.Unlock()

	for msg := range ch {
		stats.msgCounter++
		stats.byteCounter += len(msg.Payload())
		sniffer.logChan <- msg // send message to log channel
	}
}

// Display all messages in the terminal, coloured based on the topic.
// Receives messages from sniffer.logChan channel.
func (sniffer *Sniffer) logAllMessages() {
	coloursForTopicMap := map[string]string{
		"/paddle":         Cyan,
		"/playerPosition": Purple,
		"/playerBall":     Blue,
		"/opponentBall":   Green,
		"/will":           BoldRed,
		"/system/signal":  Yellow,
		"default":         Gray,
	}
	statusColour := Yellow

	for msg := range sniffer.logChan {
		topic := msg.Topic()
		payload := string(msg.Payload())

		payloadColour, exists := coloursForTopicMap[topic]
		if !exists {
			if len(topic) >= 8 && topic[:8] == "/status/" {
				payloadColour = statusColour
			} else {
				payloadColour = coloursForTopicMap["default"]
			}
		}

		fmt.Printf("%s[%s]%s %s%s%s\n",
			payloadColour, topic, Gray,
			payloadColour, payload, Gray)
	}
}

// Displays network metrics for each topic in a table format in the terminal.
func displayNetworkStats(sniffer *Sniffer) {
	ticker := time.NewTicker(2 * time.Second) // update network metrics every 2 seconds
	defer ticker.Stop()

	for range ticker.C {
		fmt.Print("\0337")
		fmt.Print("\033[24;1H")
		fmt.Print("\033[J")

		// table header
		fmt.Println(Gray + "-------------------------------------------------------" + Gray)
		fmt.Printf(BoldCyan+"%-25s %-10s %-12s %-10s %-10s %-10s\n"+BoldCyan, "TOPIC", "MSGS", "BYTES", "MSGS/s", "kbps", "AVG kbps")

		sniffer.mutex.Lock()
		tempTopicsStorage := make([]string, 0, len(sniffer.topicsNetworkStats))
		for topic := range sniffer.topicsNetworkStats {
			tempTopicsStorage = append(tempTopicsStorage, topic)
		}
		sniffer.mutex.Unlock()
		sort.Strings(tempTopicsStorage) // sort topics alphabetically

		// For each topic, calculate message rate and kbps since last update and print in table format.
		sniffer.mutex.Lock()
		for _, topic := range tempTopicsStorage {
			timeNow := time.Now()
			stats := sniffer.topicsNetworkStats[topic]
			elapsedTime := time.Since(stats.lastUpdateTime).Seconds()

			newMessages := stats.msgCounter - stats.lastMsgCounter
			msgRate := float64(newMessages) / elapsedTime

			deltaBits := float64(stats.byteCounter-stats.lastByteCounter) * 8 // convert bytes to bits
			kbps := (deltaBits / 1024.0) / elapsedTime

			totalSeconds := timeNow.Sub(stats.startTime).Seconds()
			totalBits := float64(stats.byteCounter) * 8
			avgKbps := (totalBits / 1024.0) / totalSeconds

			fmt.Printf("%-25s %-10d %-12d %-10.2f %-10.2f %-10.2f\n", topic, stats.msgCounter, stats.byteCounter, msgRate, kbps, avgKbps)

			stats.lastUpdateTime = timeNow
			stats.lastMsgCounter = stats.msgCounter
			stats.lastByteCounter = stats.byteCounter
		}
		sniffer.mutex.Unlock()
		fmt.Print(Gray + "-------------------------------------------------------" + Gray)

		fmt.Print("\0338")
	}
}

// Cleanup function to close network sniffer when program is terminated.
func (sniffer *Sniffer) closeNetworkSniffer() {
	sniffer.mutex.Lock()
	defer sniffer.mutex.Unlock()

	for topic, ch := range sniffer.topicChannels {
		close(ch)                            // close all topic channels to stop workers
		delete(sniffer.topicChannels, topic) // clean up the map
	}

	fmt.Print("\033[1;r") // Resets scroll margin to full screen
	fmt.Println("\nSniffer closed. Terminal reset.")
}

// ANSI colour codes
const (
	Red      = "\033[31m"
	Green    = "\033[32m"
	Yellow   = "\033[33m"
	Blue     = "\033[34m"
	Purple   = "\033[35m"
	Cyan     = "\033[36m"
	Gray     = "\033[37m"
	White    = "\033[97m"
	BoldRed  = "\033[1;31m"
	BoldCyan = "\033[1;36m"
)
