

package main


import (
   "bufio"
   "encoding/hex"
   "fmt"
   "log"
   "net"
   "net/http"
   "os"
   "os/exec"
   "strconv"
   "strings"

   "github.com/prometheus/client_golang/prometheus"
   "github.com/prometheus/client_golang/prometheus/promhttp"
)

// directionForEstablishedIncoming returns true if the sourcePort matches a LISTEN port (incoming ESTABLISHED)
func directionForEstablishedIncoming(sourcePort string, listenPorts map[string]struct{}) bool {
	_, ok := listenPorts[sourcePort]
	return ok
}

// interfaceCache stores the mapping of IP addresses to interface names
var interfaceCache map[string]string

// getNetworkInterfaces builds a map of IP addresses to interface names
func getNetworkInterfaces() (map[string]string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		// Handle the specific netlink error gracefully
		if strings.Contains(err.Error(), "address family not supported") || 
		   strings.Contains(err.Error(), "netlinkrib") {
			log.Printf("Warning: netlink error detected, falling back to manual interface detection: %v", err)
			return getNetworkInterfacesManual()
		}
		return nil, fmt.Errorf("failed to get network interfaces: %v", err)
	}

	ipToInterface := make(map[string]string)
	successCount := 0

	for _, iface := range interfaces {
		// Skip interfaces that are down, loopback, or point-to-point
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		// Skip certain interface types that commonly cause issues (but keep virbr and bond interfaces)
		if strings.Contains(iface.Name, "docker") || 
		   (strings.Contains(iface.Name, "veth") && !strings.Contains(iface.Name, "vnet")) ||
		   (strings.Contains(iface.Name, "br-") && !strings.Contains(iface.Name, "virbr")) {
			continue
		}

		// Explicitly include bonding interfaces (bond0, bond1, etc.)
		isBondInterface := strings.HasPrefix(iface.Name, "bond")
		if isBondInterface {
			log.Printf("Debug: Found bonding interface: %s", iface.Name)
			// Get bonding info for this interface
			bondInfo := getBondingInterfaceInfo()
			if slaves, exists := bondInfo[iface.Name]; exists {
				log.Printf("Debug: Bonding interface %s is active with slaves: %v", iface.Name, slaves)
			}
		}

		// Get addresses for this interface, but handle errors gracefully
		addrs, err := iface.Addrs()
		if err != nil {
			// Log warning but continue with other interfaces
			log.Printf("Warning: Could not get addresses for interface %s: %v", iface.Name, err)
			continue
		}

		for _, addr := range addrs {
			// Handle the address parsing more carefully
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			default:
				// Skip unknown address types
				log.Printf("Debug: Skipping unknown address type %T on interface %s", v, iface.Name)
				continue
			}

			if ip != nil {
				// Only map IPv4 addresses for now (skip IPv6 to avoid protocol issues)
				if ip.To4() != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
					ipStr := ip.String()
					
					// Check if this interface already has an IP mapped (multiple IPs scenario)
					existingIPs := []string{}
					for existingIP, existingIface := range ipToInterface {
						if existingIface == iface.Name {
							existingIPs = append(existingIPs, existingIP)
						}
					}
					
					ipToInterface[ipStr] = iface.Name
					successCount++
					
					if len(existingIPs) > 0 {
						log.Printf("Debug: Multiple IPs detected on %s - Added %s (existing: %v)", iface.Name, ipStr, existingIPs)
					}
				}
			}
		}
	}

	// If we couldn't get any interfaces, log a warning but don't fail completely
	if successCount == 0 {
		log.Printf("Warning: No usable network interfaces found, connection interface detection may be limited")
	} else {
		log.Printf("Successfully mapped %d IP addresses to network interfaces", successCount)
		// Show interface statistics and multiple IP detection
		getInterfaceStatistics(ipToInterface)
	}

	return ipToInterface, nil
}

// getNetworkInterfacesManual uses system commands as fallback when Go's net package fails
func getNetworkInterfacesManual() (map[string]string, error) {
	ipToInterface := make(map[string]string)
	
	// Use ip command to get interface information - try multiple paths
	var cmd *exec.Cmd
	var output []byte
	var err error
	
	// Try different ip command paths (systemd environment may have limited PATH)
	ipPaths := []string{"ip", "/usr/bin/ip", "/bin/ip", "/sbin/ip", "/usr/sbin/ip"}
	
	for _, ipPath := range ipPaths {
		cmd = exec.Command(ipPath, "-4", "addr", "show")
		output, err = cmd.Output()
		if err == nil {
			log.Printf("Debug: Successfully used ip command at: %s", ipPath)
			break
		}
		log.Printf("Debug: Failed to run ip at %s: %v", ipPath, err)
	}
	
	if err != nil {
		log.Printf("Warning: Could not run 'ip addr show' from any location, trying alternative method: %v", err)
		return getNetworkInterfacesFallback(), nil
	}
	
	lines := strings.Split(string(output), "\n")
	var currentInterface string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Parse interface names (e.g., "2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP>")
		if strings.Contains(line, ": <") && !strings.HasPrefix(line, "inet") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				currentInterface = strings.TrimSpace(parts[1])
			}
		}
		
		// Parse IP addresses (e.g., "inet 192.168.1.100/24 brd 192.168.1.255 scope global eth0" or "inet 172.20.164.118/24 brd 172.20.164.255 scope global secondary eth0:gssapt11")
		if strings.HasPrefix(line, "inet ") && currentInterface != "" {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ipCidr := parts[1]
				// Extract IP from CIDR notation
				if slashPos := strings.Index(ipCidr, "/"); slashPos > 0 {
					ip := ipCidr[:slashPos]
					
					// Determine interface label and type
					var interfaceLabel string
					var isSecondary bool
					
					// Check if this is a secondary IP with a label (e.g., eth0:gssapt11)
					for _, part := range parts {
						if strings.Contains(part, ":") && strings.Contains(part, currentInterface) {
							interfaceLabel = part
							break
						} else if part == "secondary" {
							isSecondary = true
						}
					}
					
					// If no specific label found, use the current interface
					if interfaceLabel == "" {
						interfaceLabel = currentInterface
					}
					
					// Skip loopback
					if ip != "127.0.0.1" && currentInterface != "lo" {
						ipToInterface[ip] = currentInterface // Always map to base interface name for consistency
						
						if strings.HasPrefix(currentInterface, "bond") {
							if isSecondary {
								log.Printf("Debug: Manual detection - Secondary IP on bonding interface: %s mapped to %s (label: %s)", ip, currentInterface, interfaceLabel)
							} else {
								log.Printf("Debug: Manual detection - Primary IP on bonding interface: %s mapped to %s", ip, currentInterface)
							}
						} else {
							if isSecondary {
								log.Printf("Debug: Manual detection - Secondary IP detected: %s mapped to %s (label: %s)", ip, currentInterface, interfaceLabel)
							} else {
								log.Printf("Debug: Manual detection - Primary IP: %s mapped to interface %s", ip, currentInterface)
							}
						}
					}
				}
			}
		}
	}
	
	log.Printf("Manual interface detection found %d IP mappings", len(ipToInterface))
	if len(ipToInterface) > 0 {
		getInterfaceStatistics(ipToInterface)
	}
	return ipToInterface, nil
}

// getBondingInterfaceInfo returns information about bonding interfaces and their slaves
func getBondingInterfaceInfo() map[string][]string {
	bondInfo := make(map[string][]string)
	
	// Check for bonding interfaces in /proc/net/bonding/
	bondDir := "/proc/net/bonding"
	if entries, err := os.ReadDir(bondDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			
			bondName := entry.Name()
			bondPath := fmt.Sprintf("%s/%s", bondDir, bondName)
			
			if content, err := os.ReadFile(bondPath); err == nil {
				slaves := []string{}
				lines := strings.Split(string(content), "\n")
				
				for _, line := range lines {
					line = strings.TrimSpace(line)
					if strings.HasPrefix(line, "Slave Interface:") {
						parts := strings.Fields(line)
						if len(parts) >= 3 {
							slaves = append(slaves, parts[2])
						}
					}
				}
				
				if len(slaves) > 0 {
					bondInfo[bondName] = slaves
					log.Printf("Debug: Bonding interface %s has slaves: %v", bondName, slaves)
				}
			}
		}
	}
	
	return bondInfo
}

// getInterfaceStatistics returns statistics about interface usage
func getInterfaceStatistics(ipToInterface map[string]string) {
	interfaceCount := make(map[string]int)
	interfaceIPs := make(map[string][]string)
	
	for ip, iface := range ipToInterface {
		interfaceCount[iface]++
		interfaceIPs[iface] = append(interfaceIPs[iface], ip)
	}
	
	log.Printf("Interface statistics:")
	for iface, count := range interfaceCount {
		if count > 1 {
			log.Printf("  %s: %d IPs (%v) - Multiple IP configuration detected", iface, count, interfaceIPs[iface])
		} else {
			log.Printf("  %s: %d IP (%v)", iface, count, interfaceIPs[iface])
		}
	}
}

// getNetworkInterfacesFallback provides hardcoded common interface mappings as last resort
func getNetworkInterfacesFallback() map[string]string {
	log.Printf("Warning: Using fallback interface detection with common defaults")
	
	// Create a basic mapping with common interface names
	ipToInterface := make(map[string]string)
	
	// Try to determine the primary interface from routing table
	var routeOutput []byte
	var routeErr error
	
	// Try different ip command paths for routing
	for _, ipPath := range []string{"ip", "/usr/bin/ip", "/bin/ip", "/sbin/ip", "/usr/sbin/ip"} {
		routeCmd := exec.Command(ipPath, "route", "show", "default")
		routeOutput, routeErr = routeCmd.Output()
		if routeErr == nil {
			break
		}
	}
	
	if routeErr == nil {
		lines := strings.Split(string(routeOutput), "\n")
		for _, line := range lines {
			if strings.Contains(line, "default via") {
				parts := strings.Fields(line)
				for i, part := range parts {
					if part == "dev" && i+1 < len(parts) {
						primaryIface := parts[i+1]
						// Assign common IP ranges to the primary interface
						ipToInterface["192.168.200.179"] = primaryIface
						
						if strings.HasPrefix(primaryIface, "bond") {
							log.Printf("Debug: Fallback - Bonding interface detected as primary: %s", primaryIface)
						} else {
							log.Printf("Debug: Fallback - Primary interface detected as %s", primaryIface)
						}
						break
					}
				}
			}
		}
	}
	
	// Add common bridge interfaces
	ipToInterface["172.16.10.1"] = "virbr2"
	ipToInterface["192.168.100.1"] = "virbr1"
	
	return ipToInterface
}

// getDetailedInterfaceInfo returns detailed information about an IP's interface assignment
func getDetailedInterfaceInfo(ip string) (interfaceName string, isSecondary bool) {
	if interfaceCache == nil {
		return "unknown", false
	}
	
	// Get the base interface name
	if iface, exists := interfaceCache[ip]; exists {
		// Check if this IP is one of multiple IPs on the same interface
		ipCount := 0
		for _, cachedIface := range interfaceCache {
			if cachedIface == iface {
				ipCount++
			}
		}
		
		// If there are multiple IPs on this interface, this might be a secondary IP
		// We can't definitively determine primary vs secondary from Go's net package alone,
		// but we can detect the multiple IP scenario
		isSecondary = ipCount > 1
		
		return iface, isSecondary
	}
	
	return "unknown", false
}

// getAvailableIPs returns list of IPs currently in the interface cache for debugging
func getAvailableIPs() []string {
	ips := make([]string, 0, len(interfaceCache))
	for ip := range interfaceCache {
		ips = append(ips, ip)
	}
	return ips
}

// getInterfaceForConnection determines the interface for a connection based on source and destination
func getInterfaceForConnection(sourceIP, destIP string) string {
	// For loopback connections, return loopback interface first
	if sourceIP == "127.0.0.1" || destIP == "127.0.0.1" {
		return "lo"
	}

	// For listening connections (destination 0.0.0.0), handle specially
	if destIP == "0.0.0.0" {
		// If source is 0.0.0.0, it's listening on all interfaces - use primary
		if sourceIP == "0.0.0.0" {
			primary := getPrimaryInterface()
			log.Printf("Debug: 0.0.0.0 listener mapped to primary interface: %s", primary)
			return primary
		}
		// If source has a specific IP, use that interface
		result := getInterfaceForIP(sourceIP)
		log.Printf("Debug: Listener on %s mapped to interface: %s", sourceIP, result)
		return result
	}

	// For established connections, prioritize source IP interface
	if sourceIP != "0.0.0.0" && sourceIP != "127.0.0.1" {
		if iface := getInterfaceForIP(sourceIP); iface != "unknown" {
			return iface
		}
	}

	// For outbound connections to external IPs, determine interface by routing
	if !isLocalIP(destIP) {
		if iface := getInterfaceForDestination(destIP); iface != "unknown" {
			return iface
		}
	}

	// Fallback to primary interface
	return getPrimaryInterface()
}

// isLocalIP checks if an IP is in private/local ranges
func isLocalIP(ip string) bool {
	if ip == "127.0.0.1" || ip == "0.0.0.0" {
		return true
	}
	
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	// Check for private IP ranges
	return parsedIP.IsLoopback() ||
		strings.HasPrefix(ip, "192.168.") ||
		strings.HasPrefix(ip, "172.16.") ||
		strings.HasPrefix(ip, "10.") ||
		strings.HasPrefix(ip, "169.254.") // Link-local
}

// getInterfaceForIP returns the interface name for a given IP address
func getInterfaceForIP(ip string) string {
	// Initialize or refresh interface cache if needed
	if interfaceCache == nil {
		var err error
		interfaceCache, err = getNetworkInterfaces()
		if err != nil {
			log.Printf("Error getting network interfaces: %v", err)
			return "unknown"
		}
	}

	// Check exact IP match first
	if iface, exists := interfaceCache[ip]; exists {
		return iface
	}

	// Debug: log when IP is not found in cache
	log.Printf("Debug: IP %s not found in cache, available IPs: %v", ip, getAvailableIPs())

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
		
		// If still not found, try to determine interface by checking if IP is in same subnet as any interface
		if iface := getInterfaceBySubnet(ip); iface != "unknown" {
			return iface
		}
	}

	return "unknown"
}

// getInterfaceForDestination determines which interface would be used for outbound connections to a destination
func getInterfaceForDestination(destIP string) string {
	// Use ip route get to determine which interface would be used
	var output []byte
	var err error
	
	// Try different ip command paths
	for _, ipPath := range []string{"ip", "/usr/bin/ip", "/bin/ip", "/sbin/ip", "/usr/sbin/ip"} {
		cmd := exec.Command(ipPath, "route", "get", destIP)
		output, err = cmd.Output()
		if err == nil {
			break
		}
	}
	
	if err != nil {
		return "unknown"
	}

	// Parse the output to extract the interface
	// Example output: "192.168.1.1 via 192.168.1.1 dev eth0 src 192.168.1.100 uid 0"
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		for i, field := range fields {
			if field == "dev" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}

	return "unknown"
}

// getInterfaceBySubnet checks if an IP belongs to any interface's subnet
func getInterfaceBySubnet(targetIP string) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}

	ip := net.ParseIP(targetIP)
	if ip == nil {
		return "unknown"
	}

	for _, iface := range interfaces {
		// Skip interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.Contains(ip) {
					return iface.Name
				}
			}
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

	// First priority: bonding interfaces (bond0, bond1, etc.) as they're typically primary in enterprise
	for _, iface := range interfaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		
		// Highest priority: bonding interfaces
		if strings.HasPrefix(iface.Name, "bond") {
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			// Check if it has an IPv4 address
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
					log.Printf("Debug: Selected bonding interface %s as primary", iface.Name)
					return iface.Name
				}
			}
		}
	}

	// Second priority: ethernet interfaces (eth0, en*, etc.)
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
	   "network_connections_info",
	   "Information about network connections",
	   []string{"source_address", "source_port", "destination_address", "destination_port", "state", "interface", "protocol", "direction", "process_name"},
	   nil,
	  ),
	 }
}

func (c *networkConnectionsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.metric
}

func (c *networkConnectionsCollector) Collect(ch chan<- prometheus.Metric) {
	// Build set of LISTEN ports for direction classification
	listenPorts := make(map[string]struct{})
	tcpConnectionsRaw, err := getTCPConnections("/proc/net/tcp", nil)
	if err == nil {
		for _, conn := range tcpConnectionsRaw {
			if conn.state == "LISTEN" {
				listenPorts[conn.sourcePort] = struct{}{}
			}
		}
	}

	// Collect TCP connections with direction label
	tcpConnections, err := getTCPConnections("/proc/net/tcp", listenPorts)
	if err != nil {
		log.Printf("Error getting TCP connections: %v", err)
	} else {
		for _, conn := range tcpConnections {
			direction := "outgoing"
			if _, ok := listenPorts[conn.sourcePort]; ok {
				direction = "incoming"
			}
			ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, 1, conn.sourceAddress, conn.sourcePort, conn.destinationAddress, conn.destinationPort, conn.state, conn.sourceInterface, "tcp", direction, conn.processName)
		}
	}

	// Collect UDP sockets (no direction logic for now)
	udpConnections, err := getUDPConnections("/proc/net/udp")
	if err != nil {
		log.Printf("Error getting UDP connections: %v", err)
	} else {
		 for _, conn := range udpConnections {
		  ch <- prometheus.MustNewConstMetric(c.metric, prometheus.GaugeValue, 1, conn.sourceAddress, conn.sourcePort, conn.destinationAddress, conn.destinationPort, conn.state, conn.sourceInterface, "udp", "unknown", "")
		 }
	}
}

type tcpConnection struct {
	sourceAddress      string
	sourcePort         string
	destinationAddress string
	destinationPort    string
	state              string
	sourceInterface    string
	 processName       string
}

func getTCPConnections(file string, listenPorts map[string]struct{}) ([]tcpConnection, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Build a map of (localIP, localPort) to process name using ss -tup
	cmd := exec.Command("ss", "-tulnp")
	output, err := cmd.Output()
	listenProcMap := make(map[string]string) // port -> process name
	if err == nil {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) < 6 {
				continue
			}
			state := fields[1]
			local := fields[4]
			procInfo := fields[len(fields)-1]
			// Only consider LISTEN sockets
			if state == "LISTEN" && strings.Contains(procInfo, "users:(") {
				start := strings.Index(procInfo, "(")
				end := strings.Index(procInfo, ")")
				if start != -1 && end != -1 && end > start {
					procDetails := procInfo[start+1 : end]
					procName := strings.Split(procDetails, ",")[0]
					procName = strings.Trim(procName, "()[]{} ") // Remove brackets, parentheses, spaces
					procName = strings.ReplaceAll(procName, "\"", "") // Remove all quotes
					// Extract port
					port := ""
					if strings.HasPrefix(local, "[") {
						// IPv6 [::]:PORT
						idx := strings.LastIndex(local, ":")
						if idx != -1 {
							port = local[idx+1:]
						}
					} else if strings.HasPrefix(local, "*:") {
						port = strings.Split(local, ":")[1]
					} else {
						parts := strings.Split(local, ":")
						if len(parts) == 2 {
							port = parts[1]
						} else if len(parts) > 2 {
							port = parts[len(parts)-1]
						}
					}
					if port != "" {
						listenProcMap[port] = procName
					}
				}
			}
		}
	}

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

		// Assign process name for LISTEN and ESTABLISHED incoming connections
		processName := ""
		isListen := connectionState(state) == "LISTEN"
		isEstablishedIncoming := connectionState(state) == "ESTABLISHED" && listenPorts != nil && directionForEstablishedIncoming(sourcePort, listenPorts)
		if isListen || isEstablishedIncoming {
			if name, ok := listenProcMap[sourcePort]; ok {
				processName = name
			}
		}


		connections = append(connections, tcpConnection{
			sourceAddress:      sourceAddress,
			sourcePort:         sourcePort,
			destinationAddress: destinationAddress,
			destinationPort:    destinationPort,
			state:              connectionState(state),
			sourceInterface:    getInterfaceForConnection(sourceAddress, destinationAddress),
			processName:        processName,
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

// getUDPConnections parses UDP sockets from /proc/net/udp
func getUDPConnections(file string) ([]tcpConnection, error) {
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
		// UDP sockets don't have traditional states, but we can use the socket state
		// For UDP, we'll use "LISTEN" for bound sockets and "UNCONN" for unconnected
		state := "UNCONN" // Default for UDP
		
		// Check if it's a bound UDP socket (local port is not 0)
		sourceAddress, sourcePort, err := parseAddress(localAddress)
		if err != nil {
			log.Printf("Error parsing local address: %v", err)
			continue
		}
		
		// For UDP, if there's a local address bound, consider it "listening"
		if sourcePort != "0" {
			state = "LISTEN"
		}

		destinationAddress, destinationPort, err := parseAddress(remoteAddress)
		if err != nil {
			log.Printf("Error parsing remote address: %v", err)
			continue
		}

		// Get network interface for source IP (use same logic as TCP connections)
		sourceInterface := getInterfaceForConnection(sourceAddress, destinationAddress)

		connections = append(connections, tcpConnection{
			sourceAddress:      sourceAddress,
			sourcePort:         sourcePort,
			destinationAddress: destinationAddress,
			destinationPort:    destinationPort,
			state:              state,
			sourceInterface:    sourceInterface,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return connections, nil
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
