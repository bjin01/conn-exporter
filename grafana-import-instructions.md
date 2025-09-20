# Grafana Dashboard Import Instructions

## How to Import the Network Connections Dashboard

### Method 1: Import via JSON File
1. Open Grafana web interface
2. Go to "Dashboards" > "New" > "Import"
3. Click "Upload JSON file" and select `grafana-dashboard.json`
4. Configure the data source (select your Prometheus instance)
5. Click "Import"

### Method 2: Import via Dashboard ID (if uploaded to grafana.com)
1. Go to "Dashboards" > "New" > "Import"
2. Enter dashboard ID or paste the JSON content
3. Configure data source
4. Click "Import"

## Dashboard Features

### 📊 **Panels Included:**

1. **Network Connections Table** (Main Panel)
   - Shows all connections with: Source Address, Source Port, Destination Address, Destination Port, State, Interface
   - Color-coded connection states
   - Sortable columns

2. **Connections by State** (Time Series)
   - Line graph showing connection counts by state over time
   - Tracks ESTABLISHED, LISTEN, TIME_WAIT, etc.

3. **Connections by Interface** (Time Series)
   - Line graph showing connection distribution across network interfaces
   - Helps identify interface usage patterns

4. **Top 10 Source Ports** (Table)
   - Shows most active source ports
   - Useful for identifying services

5. **Top 10 Destination Ports** (Table)
   - Shows most targeted destination ports
   - Helps identify popular services

6. **Connection State Distribution** (Pie Chart)
   - Visual distribution of connection states
   - Quick overview of connection health

7. **Established Connections Only** (Filtered Table)
   - Shows only active ESTABLISHED connections
   - Cleaner view of active traffic

8. **Listening Ports** (Table)
   - Shows all services listening for connections
   - Security and service monitoring

### 🎛️ **Dashboard Variables:**
- **Interface Filter**: Select specific network interfaces to monitor
- **State Filter**: Filter by connection state (ESTABLISHED, LISTEN, etc.)

### ⚙️ **Configuration:**
- **Refresh Rate**: 5 seconds (adjustable)
- **Time Range**: Last 5 minutes (adjustable)
- **Data Source**: Requires Prometheus with conn-exporter metrics

## Prerequisites

1. **Prometheus Data Source** configured in Grafana
2. **conn-exporter** running and being scraped by Prometheus
3. Metrics available at: `network_connections_info`

## Troubleshooting

### No Data Showing:
- Verify Prometheus data source is configured correctly
- Check that conn-exporter is running on port 9100
- Ensure Prometheus is scraping the conn-exporter target
- Verify metrics are available: `curl http://localhost:9100/metrics`

### Panel Errors:
- Check the Prometheus query syntax
- Verify metric name `network_connections_info` exists
- Check time range settings

### Performance Issues:
- Adjust refresh rate if too frequent
- Reduce time range for faster queries
- Consider adding filters to reduce data volume

## Customization

### Adding New Panels:
- Use the provided PromQL queries from `promql-queries.txt`
- Common query patterns:
  ```promql
  # Filter by interface
  network_connections_info{interface="eth0"}
  
  # Filter by state
  network_connections_info{state="ESTABLISHED"}
  
  # Aggregate by labels
  sum(network_connections_info) by (interface, state)
  ```

### Modifying Existing Panels:
- Edit panel queries to focus on specific interfaces or states
- Adjust visualization types (table → graph, etc.)
- Modify color schemes and thresholds

## Dashboard URL Structure
Once imported, the dashboard will be available at:
`http://your-grafana-host:3000/d/conn-exporter/network-connections-monitor`