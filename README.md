# 🌍 intspeed

Has ISP ever sold you international connectivity for high price but performance
keeps throttling? Here's tool made to prove it. International network performance
testing tool that measures your connection speed to servers around the globe.

## Quick Start

### Download Latest Release

```bash
# Download CLI tool
curl -L -o intspeed https://github.com/rotkonetworks/intspeed/releases/latest/download/intspeed
chmod +x intspeed
sudo mv intspeed /usr/local/bin/

# Download web server (optional)
curl -L -o intspeed-server https://github.com/rotkonetworks/intspeed/releases/latest/download/intspeed-server
chmod +x intspeed-server
sudo mv intspeed-server /usr/local/bin/
```

### Verify Downloads (Recommended)

```bash
# Download signature files
curl -L -o intspeed.sig https://github.com/rotkonetworks/intspeed/releases/latest/download/intspeed.sig
curl -L -o intspeed.sha512 https://github.com/rotkonetworks/intspeed/releases/latest/download/intspeed.sha512

# Verify checksum
sha512sum -c intspeed.sha512

# Import GPG key and verify signature
curl -L https://rotko.net/pgp | gpg --import
gpg --verify intspeed.sig intspeed
```

## Usage

### CLI Tool

```bash
# Run speed tests to all global locations
intspeed test

# Generate HTML report
intspeed test --html

# List all test locations
intspeed locations

# Generate HTML from existing results
intspeed html results/latest.json

# Customize output
intspeed test --output /tmp/speedtest --threads 4 --timeout 120 --verbose
```

### Web Interface

```bash
# Start interactive web server
intspeed-server

# Custom port
intspeed-server --port 3000
```

Then open http://localhost:8080 in your browser.

## Global Test Locations

- **North America**: New York, Los Angeles, Chicago, Toronto, Vancouver
- **Europe**: London, Frankfurt, Amsterdam, Stockholm, Paris  
- **Asia-Pacific**: Tokyo, Seoul, Singapore, Hong Kong, Sydney
- **Middle East**: Dubai
- **South America**: São Paulo

## Installation from Source

### Requirements
- Go 1.21 or later

### Build
```bash
git clone https://github.com/rotkonetworks/intspeed.git
cd intspeed
make build

# Install globally
make install
```

### Development

```bash
# With Nix (recommended)
nix develop

# With Go
go mod download
make build
```

## Output

Results are saved as JSON files in the `results/` directory:
- `results_YYYY-MM-DD_HH-MM-SS.json` - Timestamped results
- `latest.json` - Most recent test results

### Sample Output

```
🌍 intspeed v2.0.0 from rotko.net
📍 Testing 17 locations sequentially for accurate results
⏱️  Estimated time: 51 minutes

✅ [1/17] New York (3/5 ISPs): Best: Verizon 12.3ms ↓847.2/↑156.4 | Avg: 15.8ms ↓692.1/↑134.2 Mbps
✅ [2/17] London (4/5 ISPs): Best: BT 45.6ms ↓523.1/↑89.3 | Avg: 52.1ms ↓445.8/↑76.8 Mbps
...

📊 Results saved: results/results_2025-06-10_14-25-13.json
✅ Success: 15/17 (88.2%)
📊 Avg: 67.4ms, ↓412.6 Mbps, ↑98.3 Mbps
🏆 Best latency: Tokyo → NTT (8.9ms)
🏆 Best download: New York → Verizon (847.2 Mbps)
```

## Features

- **Sequential Testing**: Tests locations one by one for accurate results
- **Multiple ISPs**: Tests multiple ISPs per location for comprehensive coverage
- **Real-time Progress**: Live updates during testing
- **Rich Output**: JSON results, HTML reports, and terminal summaries
- **Web Interface**: Interactive browser-based testing
- **Cryptographic Verification**: GPG-signed releases with checksums

## License

MIT License - see LICENSE file for details.

## About

Created by [rotko.net](https://rotko.net) for accurate international network performance measurement.
