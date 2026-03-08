# Wayzn Reverse Engineering Toolkit

Control your Wayzn Smart Pet Door from your own web app instead of the official mobile app.

## How It Works

Since Wayzn has no public API, this toolkit helps you:
1. **Capture** the Wayzn app's API traffic using a MITM proxy
2. **Analyze** the captured traffic to identify endpoints and auth patterns
3. **Control** your door via a simple web UI or Go API client

## Step 1: Capture Traffic

### Prerequisites
- Python 3.10+ with `mitmproxy` installed: `pip install mitmproxy`
- A computer on the same WiFi network as your phone
- Your phone configured to use the MITM proxy

### Setup

```bash
# Start the capture proxy
cd capture/
mitmdump -s wayzn_capture.py -p 8080
```

### Configure Your Phone

**Android:**
1. Go to Settings > WiFi > long-press your network > Modify > Advanced
2. Set Proxy to Manual, enter your computer's IP and port 8080
3. Open `http://mitm.it` in your browser and install the mitmproxy CA certificate
4. For Android 7+, you may need to add the cert to the system trust store (requires root or use an older Android device/emulator)

**iOS:**
1. Go to Settings > WiFi > tap (i) on your network
2. Configure Proxy > Manual, enter your computer's IP and port 8080
3. Open `http://mitm.it` in Safari and install the profile
4. Go to Settings > General > About > Certificate Trust Settings and enable the mitmproxy cert

### Capture API Calls

1. Open the Wayzn app on your phone
2. Tap "Open" and "Close" a few times
3. Check the door status
4. The proxy logs all Wayzn-related traffic to `capture/wayzn_api_log.json`

**Discovery mode** (if no Wayzn traffic is detected):
```bash
WAYZN_CAPTURE_ALL=true mitmdump -s wayzn_capture.py -p 8080
```
This captures ALL traffic so you can identify which domains Wayzn actually uses.

## Step 2: Analyze Traffic

```bash
cd capture/

# View a summary of captured endpoints
python analyze.py

# Auto-generate a Go client from captured traffic
python analyze.py --generate-go
```

The analyzer identifies:
- API hosts and endpoints
- Authentication headers and tokens
- Open/close/status command patterns

## Step 3: Configure and Run

### Create your config

Copy the example config and fill in values from the traffic analysis:

```bash
cp wayzn.example.json wayzn.json
```

Edit `wayzn.json` with the base URL, auth token, and device ID discovered in Step 2.

### Build and run the web server

```bash
cd cmd/wayzn-server/
go build -o wayzn-server .
./wayzn-server -config ../../wayzn.json -addr :8080
```

Open `http://localhost:8080` in your browser. You'll see Open/Close buttons and status display.

### API Endpoints

The web server exposes a simple REST API:

| Method | Path          | Description        |
|--------|---------------|--------------------|
| POST   | `/api/open`   | Open the door      |
| POST   | `/api/close`  | Close the door     |
| GET    | `/api/status` | Get door status    |

Example with curl:
```bash
curl -X POST http://localhost:8080/api/open
curl -X POST http://localhost:8080/api/close
curl http://localhost:8080/api/status
```

## Project Structure

```
wayzn/
  capture/
    wayzn_capture.py    # mitmproxy script to intercept Wayzn traffic
    analyze.py          # Analyze captured traffic, generate Go client
    wayzn_api_log.json  # Captured API calls (gitignored)
  cmd/
    wayzn-server/       # Web server with Open/Close UI
  pkg/
    wayzn/              # Go API client library
      client.go         # Client core (auth, HTTP, config)
      endpoints.go      # Open/Close/Status commands (stubs)
  wayzn.example.json    # Example config file
```

## Tips

- **Token expiry**: Auth tokens may expire. If commands stop working, re-capture traffic from the app to get a fresh token.
- **Certificate pinning**: If the Wayzn app uses certificate pinning, you'll need to bypass it. On a rooted Android device, use Frida with `frida-android-unpinning`. On iOS, use SSL Kill Switch 2.
- **Alexa integration**: If you have Alexa linked, the Alexa Smart Home API calls may reveal additional endpoints.
- **Alternative approach**: If MITM capture fails due to cert pinning, try decompiling the APK (`com.wayzn.android`) with JADX to find hardcoded API URLs.

## Security

- Never commit `wayzn.json` (contains your auth token)
- The web server has no authentication — only run it on trusted networks
- Add basic auth or run behind a reverse proxy for production use
