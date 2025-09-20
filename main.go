
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
		// Skip interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Get addresses for this interface, but handle errors gracefully
		addrs, err := iface.Addrs()
		if err != nil {
			// Log warning but continue with other interfaces
			log.Printf("Warning: Could not get addresses for interface %s: %v", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				// Skip unknown address types
				continue
			}

			if ip != nil {
				// Only map IPv4 addresses for now (IPv6 ready for future)
				if ip.To4() != nil && !ip.IsLoopback() {
					ipToInterface[ip.String()] = iface.Name
				}
			}
		}
	}

	return ipToInterface, nil
}

// getInterfaceForIP returns the interface name for a given IP address
func getInterfaceForIP(ip string) string {
	// Initialize or refresh interface cache if needed
	if interfaceCache == nil {
		var err error
		interfaceCache, err = getNetworkInterfaces()
		if err != nil {
			log.Printf("Error getting network interfaces: %v", err)
			// Return "unknown" but don't crash the whole program
			return "unknown"
		}
	}

	// Check exact IP match first
	if iface, exists := interfaceCache[ip]; exists {
		return iface
	}

	// Handle special addresses
	switch ip {
	case "127.0.0.1":
		return "lo"
	case "0.0.0.0":
		// For 0.0.0.0 (listen on all), find the primary interface
		// Try to find the default route interface or first non-loopback interface
		return getPrimaryInterface()
	default:
		// If IP not found in cache, try to refresh the cache once
		// This handles dynamic interface changes (containers, etc.)
		var err error
		interfaceCache, err = getNetworkInterfaces()
		if err != nil {
			log.Printf("Error refreshing network interfaces: %v", err)
			return "unknown"
		}
		
		// Check again after refresh
		if iface, exists := interfaceCache[ip]; exists {
			return iface
		}
	}

	return "unknown"
}

// getPrimaryInterface returns the primary network interface name
func getPrimaryInterface() string {
	// Try to find interfaces with IPv4 addresses (excluding loopback)
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}

	// First, try to find the interface with a default gateway (eth0, etc.)
	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		// Prefer ethernet interfaces
		if strings.HasPrefix(iface.Name, "eth") || strings.HasPrefix(iface.Name, "en") {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			// Check if it has an IPv4 address
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
					return iface.Name
				}
			}
		}
	}

	// Fallback: return first up interface with IPv4 (excluding loopback)
	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
				return iface.Name
			}
		}
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

	// Get port from environment variable or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "9100"
	}
	
	http.Handle("/metrics", promhttp.Handler())
	log.Printf("Beginning to serve on port :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
