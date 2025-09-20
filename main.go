
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// interfaceCache stores the mapping of IP addresses to interface names
var interfaceCache map[string]string

// getNetworkInterfaces builds a map of IP addresses to interface names
func getNetworkInterfaces() (map[string]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ipToInterface := make(map[string]string)

	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil {
				// Only map IPv4 addresses for now
				if ip.To4() != nil {
					ipToInterface[ip.String()] = iface.Name
				}
			}
		}
	}

	return ipToInterface, nil
}

// getInterfaceForIP returns the interface name for a given IP address
func getInterfaceForIP(ip string) string {
	if interfaceCache == nil {
		var err error
		interfaceCache, err = getNetworkInterfaces()
		if err != nil {
			log.Printf("Error getting network interfaces: %v", err)
			return "unknown"
		}
	}

	if iface, exists := interfaceCache[ip]; exists {
		return iface
	}

	// For special addresses
	if ip == "0.0.0.0" || ip == "127.0.0.1" {
		return "lo"
	}

	return "unknown"
}

const (
	namespace = "network"
	subsystem = "connections"
)

type networkConnectionsCollector struct {
	metric *prometheus.Desc
}

func newNetworkConnectionsCollector() *networkConnectionsCollector {
	return &networkConnectionsCollector{
		metric: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, subsystem, "info"),
			"Network connection information",
			[]string{"source_address", "source_port", "destination_address", "destination_port", "state", "interface"},
			nil,
		),
	}
}

func (c *networkConnectionsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metric
}

func (c *networkConnectionsCollector) Collect(ch chan<- prometheus.Metric) {
	tcpConnections, err := getTCPConnections("/proc/net/tcp")
	if err != nil {
		log.Printf("Error getting TCP connections: %v", err)
	} else {
		for _, conn := range tcpConnections {
			ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, 1, conn.sourceAddress, conn.sourcePort, conn.destinationAddress, conn.destinationPort, conn.state, conn.sourceInterface)
		}
	}

	// IPv6 support commented out for future use
	/*
	tcp6Connections, err := getTCPConnections("/proc/net/tcp6")
	if err != nil {
		log.Printf("Error getting TCP6 connections: %v", err)
	} else {
		for _, conn := range tcp6Connections {
			ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, 1, conn.sourceAddress, conn.sourcePort, conn.destinationAddress, conn.destinationPort, conn.state, conn.sourceInterface)
		}
	}
	*/
}

type tcpConnection struct {
	sourceAddress      string
	sourcePort         string
	destinationAddress string
	destinationPort    string
	state              string
	sourceInterface    string
}

func getTCPConnections(file string) ([]tcpConnection, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var connections []tcpConnection

	scanner := bufio.NewScanner(f)
	scanner.Scan() // Skip header line

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		localAddress := fields[1]
		remoteAddress := fields[2]
		state := fields[3]

		sourceAddress, sourcePort, err := parseAddress(localAddress)
		if err != nil {
			log.Printf("Error parsing local address: %v", err)
			continue
		}

		destinationAddress, destinationPort, err := parseAddress(remoteAddress)
		if err != nil {
			log.Printf("Error parsing remote address: %v", err)
			continue
		}

		connections = append(connections, tcpConnection{
			sourceAddress:      sourceAddress,
			sourcePort:         sourcePort,
			destinationAddress: destinationAddress,
			destinationPort:    destinationPort,
			state:              connectionState(state),
			sourceInterface:    getInterfaceForIP(sourceAddress),
		})
	}

	return connections, nil
}

func parseAddress(addr string) (string, string, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid address format: %s", addr)
	}

	ipBytes, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", "", err
	}

	port, err := strconv.ParseInt(parts[1], 16, 64)
	if err != nil {
		return "", "", err
	}

	var ip net.IP
	if len(ipBytes) == 4 { // IPv4
		// Reverse byte order for IPv4 (little-endian to big-endian)
		reversedBytes := make([]byte, 4)
		reversedBytes[0] = ipBytes[3]
		reversedBytes[1] = ipBytes[2]
		reversedBytes[2] = ipBytes[1]
		reversedBytes[3] = ipBytes[0]
		ip = net.IP(reversedBytes).To4()
	} else if len(ipBytes) == 16 { // IPv6 - commented out for future use
		/*
		// For IPv6, reverse each 4-byte segment
		reversedBytes := make([]byte, 16)
		for i := 0; i < 4; i++ {
			reversedBytes[i*4] = ipBytes[i*4+3]
			reversedBytes[i*4+1] = ipBytes[i*4+2]
			reversedBytes[i*4+2] = ipBytes[i*4+1]
			reversedBytes[i*4+3] = ipBytes[i*4]
		}
		ip = net.IP(reversedBytes).To16()
		*/
		return "", "", fmt.Errorf("IPv6 support is currently disabled")
	} else {
		return "", "", fmt.Errorf("invalid IP address length: %d", len(ipBytes))
	}

	return ip.String(), strconv.FormatInt(port, 10), nil
}

func connectionState(s string) string {
	switch s {
	case "01":
		return "ESTABLISHED"
	case "02":
		return "SYN_SENT"
	case "03":
		return "SYN_RECV"
	case "04":
		return "FIN_WAIT1"
	case "05":
		return "FIN_WAIT2"
	case "06":
		return "TIME_WAIT"
	case "07":
		return "CLOSE"
	case "08":
		return "CLOSE_WAIT"
	case "09":
		return "LAST_ACK"
	case "0A":
		return "LISTEN"
	case "0B":
		return "CLOSING"
	default:
		return "UNKNOWN"
	}
}

func main() {
	collector := newNetworkConnectionsCollector()
	prometheus.MustRegister(collector)

	http.Handle("/metrics", promhttp.Handler())
	log.Println("Beginning to serve on port :9100")
	log.Fatal(http.ListenAndServe(":9100", nil))
}
