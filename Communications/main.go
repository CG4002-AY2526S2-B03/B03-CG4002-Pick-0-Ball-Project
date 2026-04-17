// SSH Reverse Tunnel into our group's Ultra96, and opens port 8883 for MQTT.
// Runs the MQTT Client script on Ultra96, and copies output logs from Ultra96 in the terminal.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"
)

const ultra96Password = "raspberry."
const ultra96IP = "172.26.191.218"

const mqttBroker = "172.20.33.183"
const mqttPort = 8883

func main() {
	// Properly handle user termination
	terminateChan := make(chan os.Signal, 1) // Channel receiving OS signal (Ctrl+C)
	signal.Notify(terminateChan, os.Interrupt, syscall.SIGTERM)

	// Goroutine that handles MQTT network sniffing and metrics calculation
	snifferInstance := startNetworkSniffer()
	defer snifferInstance.closeNetworkSniffer() // when main() exits, close the sniffer automatically

	// Handle SSH connection to Ultra96, retry until successful.
	var sshConn *ssh.Client
SSHLoop:
	for {
		var err error
		sshConn, err = setupSSHConnection()
		if err == nil {
			// properly close any existing connections on port 8883, then set up the tunnel
			cleanupSession, err := sshConn.NewSession()
			if err == nil {
				cleanupSession.Run("echo '" + ultra96Password + "' | sudo -S fuser -k 8883/tcp")
				cleanupSession.Close()
			}
			err = setupTunnelWithRetry(sshConn)
			if err == nil {
				break SSHLoop
			}
			log.Printf("SSH Connected, but Tunnel failed: %v. Retrying everything...", err)
			sshConn.Close()
		} else {
			log.Println("Failed to set up SSH connection: ", err)
		}

		select {
		case <-terminateChan:
			fmt.Println("\nTermination signal received during SSH retry.")
			return // Quits Go program
		case <-time.After(3 * time.Second):
		}
	}

	startSystemCoordinator()                     // blocks until all devices are ready, then publishes "START" signal to Ultra96
	defer closeSystemCoordinator(sysCoordClient) // ensure cleanup when main() exits

	go func() {
		runU96Script(sshConn, terminateChan)
		terminateChan <- syscall.SIGTERM // If U96 script crashes, trigger shutdown
	}()

	// main() blocks until an OS signal (Ctrl+C) is received
	<-terminateChan
	cleanup(sshConn)
}

// Sets up an SSH connection to the Ultra96, using password authentication.
func setupSSHConnection() (*ssh.Client, error) {
	// Configure ULtra96 as an SSH client
	u96Config := &ssh.ClientConfig{
		Timeout: 5 * time.Second,
		User:    "xilinx",
		Auth: []ssh.AuthMethod{
			ssh.Password(ultra96Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Connect to the SSH server
	fmt.Println("REMINDER: Turn on SOC VPN to connect to Ultra96 via SSH")
	hostAddress := net.JoinHostPort(ultra96IP, "22")
	return ssh.Dial("tcp", hostAddress, u96Config)
}

// Handles the reverse SSH tunnel.
// Tells the Ultra96 to listen on port 8883 and forward any traffic through the SSH connection to the local MQTT broker.
func setupTunnelWithRetry(conn *ssh.Client) error {
	// Attempt to listen. If fails, the SSHLoop in main() will catch it.
	remoteListener, err := conn.Listen("tcp", "127.0.0.1:8883")
	if err != nil {
		return err
	}

	// Remote Port Forwarding: opens port on U96, any connection to that port will be captured by this listener.
	go func() {
		defer remoteListener.Close() // ensure listener is closed when function exits
		for {
			remoteConn, err := remoteListener.Accept() // blocking until client.connect() is made on U96:8883
			if err != nil {
				log.Printf("Accept connection failed: %v", err)
				return
			}

			localConn, err := net.Dial("tcp", "localhost:8883") // Go program dials actual MQTT broker
			if err != nil {
				log.Printf("Dial to local MQTT broker failed: %v", err)
				return
			}

			// GOROUTINE BRIDGE: Create two-way data pipe between remoteConn (U96:8883) and localConn (Broker:8883).
			go func(r, l net.Conn) {
				defer r.Close()
				defer l.Close()
				go io.Copy(l, r) // copies data from U96 to Broker
				io.Copy(r, l)    // copies data from Broker to U96
			}(remoteConn, localConn)
		}
	}()

	log.Println("[SSH] Remote tunnel established on port 8883")
	return nil
}

// Handler that runs the MQTT client script on the Ultra96 via the SSH session.
// Output logs from the script are printed in the terminal.
func runU96Script(conn *ssh.Client, terminateChan chan<- os.Signal) {
	// Create an SSH session, in order to execute commands on the Ultra96
	session, err := conn.NewSession()
	if err != nil {
		log.Println("[ERR] unable to create session: ", err)
		terminateChan <- syscall.SIGTERM // If program fails to create session, trigger shutdown
		return
	}
	defer session.Close()

	// Get session pipes. If program fails to get any pipe, trigger shutdown.
	stdoutBuf, err := session.StdoutPipe()
	if err != nil {
		log.Println("[ERR] Failed to get stdout pipe: ", err)
		terminateChan <- syscall.SIGTERM
		return
	}
	stdinBuf, err := session.StdinPipe()
	if err != nil {
		log.Println("[ERR] Failed to get stdin pipe: ", err)
		terminateChan <- syscall.SIGTERM
		return
	}
	stderrBuf, err := session.StderrPipe()
	if err != nil {
		log.Println("[ERR] Failed to get stderr pipe: ", err)
		terminateChan <- syscall.SIGTERM
		return
	}

	err = session.Shell()
	if err != nil {
		log.Println("[ERR] failed to start shell: ", err)
		terminateChan <- syscall.SIGTERM
		return
	}

	// 2 Goroutines to continuously read from stdout and stderr of the SSH session and print logs in terminal
	go func() {
		scanner := bufio.NewScanner(stdoutBuf) // buffered scanner
		for scanner.Scan() {
			fmt.Println(scanner.Text())
		}
	}()
	go func() {
		scanner := bufio.NewScanner(stderrBuf)
		for scanner.Scan() {
			fmt.Println(" [! U96 ERR] " + scanner.Text())
		}
	}()

	cmds := []string{"echo 'raspberry.' | sudo -S -E python3 -u ~/comms_v3/ai_u96_client.py"}
	for _, cmd := range cmds {
		stdinBuf.Write([]byte(cmd + "\n"))
	}

	err = session.Wait()
	if err != nil {
		log.Println("Command execution failed: ", err)
		terminateChan <- syscall.SIGTERM // If U96 script crashes, trigger shutdown
		return
	}

}

// Cleanup function to close SSH connection when program is terminated.
func cleanup(sshClient *ssh.Client) {
	fmt.Println("Gracefully shutting down Go program")
	sshClient.Close()
}
