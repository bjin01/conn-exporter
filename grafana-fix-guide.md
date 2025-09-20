# Grafana Dashboard Data Source Fix

## Problem: Panels Not Linked to Data Source

The issue was that the dashboard JSON had hardcoded data source UIDs that don't match your Prometheus instance.

## Solution: Use the Fixed Dashboard

I've created `grafana-dashboard-fixed.json` which includes:

### ✅ **Fixed Issues:**
1. **Data Source Variables**: Uses `${DS_PROMETHEUS}` instead of hardcoded UIDs
2. **Input Mapping**: Added `__inputs` section for proper data source mapping
3. **Requirements**: Added `__requires` section for compatibility
4. **Variable Filters**: Enhanced queries to work with dashboard variables

### 🔧 **Import Instructions:**

#### Method 1: Import Fixed Dashboard
1. Delete the old dashboard (if imported)
2. Go to **Dashboards** → **New** → **Import**
3. Upload `grafana-dashboard-fixed.json`
4. **Select your Prometheus data source** when prompted
5. Click **Import**

#### Method 2: Manual Data Source Fix (for existing dashboard)
1. Go to your existing dashboard
2. Click **Dashboard settings** (gear icon)
3. Go to **Variables** tab
4. For each variable (`interface` and `state`):
   - Click **Edit**
   - Change **Data source** to your Prometheus instance
   - Click **Update**
5. For each panel:
   - Click **Edit** (pencil icon)
   - In **Query** tab, change **Data source** to your Prometheus instance
   - Click **Apply**

### 🔍 **Verification Steps:**

1. **Check Variables Work:**
   - Look for **Interface** and **State** dropdowns at the top
   - They should populate with your actual data

2. **Check Panels Show Data:**
   - Main table should show connections
   - Graphs should display time series data

3. **Test Filters:**
   - Try selecting different interfaces (eth0, lo, etc.)
   - Try filtering by state (ESTABLISHED, LISTEN, etc.)

### 🛠️ **Troubleshooting:**

#### No Data in Panels:
```bash
# 1. Verify conn-exporter is running
curl http://localhost:9100/metrics | grep network_connections_info

# 2. Check Prometheus is scraping
# Go to Prometheus UI → Status → Targets
# Look for conn-exporter target

# 3. Test query in Prometheus
# Go to Prometheus UI → Graph
# Query: network_connections_info
```

#### Variables Not Populating:
- Go to **Dashboard Settings** → **Variables**
- Click **Test** next to each variable
- Ensure queries return data

#### Panel Errors:
- Edit panel
- Check **Query** tab for red error messages
- Verify data source is selected
- Test query in Prometheus UI first

### 📊 **Dashboard Features (Fixed):**

1. **Network Connections Table** - Main comprehensive view
2. **Connections by State** - Time series graph
3. **Connections by Interface** - Interface distribution
4. **Top Source/Destination Ports** - Popular ports analysis
5. **Connection State Distribution** - Pie chart overview
6. **Filtered Views** - Established and listening connections

### 🎯 **Quick Test:**

After importing the fixed dashboard:
1. Check if you see network interface options in the **Interface** dropdown
2. Verify the main table shows your connections with proper source/destination info
3. Confirm graphs update when you change the time range

If you still have issues, the problem might be:
- Prometheus not scraping conn-exporter
- conn-exporter not running
- Wrong Prometheus data source configuration in Grafana