# OBD Bridge service

The `mgu-obd-adapter` service is the native companion for OBD Bridge
`1.1.3`. It provides ENET discovery on UDP 6811 and diagnostic transport on
TCP 6801 over the car hotspot. The committed Linux amd64 binary is built from
this directory with the Go standard library.

Version `1.1.3` supports the observed bootmod3 (BM3) sequence and adds HSFZ
keepalive handling for experimental BimmerLink use in ENET Wi-Fi mode. The
real gateway handles its own keepalives. The virtual TASU gateway acknowledges
addressed keepalive queries locally and consumes keepalive responses without
sending them to the TASU provider.

## Security boundary

- Listeners bind to the selected hotspot interface. Non-bridge wireless
  interfaces must report AP, AP-VLAN, or P2P-GO mode.
- Only same-subnet peers are accepted, and one external client owns the
  diagnostic connection at a time.
- Tagged nftables rules permit only the required hotspot and vehicle traffic.
  They are reconciled after firewall reloads and removed when the service is
  disabled or stopped.
- Administrative HTTP is available only through the group-restricted
  `/run/mgu-obd-adapter/control.sock` Unix socket.
- Frames, queues, discovery attempts, and timeouts are bounded. Requests are
  never replayed automatically after a lost connection.
- Status and diagnostic exports exclude VINs, serial numbers, and raw vehicle
  replies.

The forwarding policy permits UDS services `01`, `09`, `19`, `1A`,
`21`, and `22`; default or extended diagnostic sessions; tester-present;
and a small allowlist of BMW read setup operations required for DME metadata.
The allowlisted dynamic-DID windows and metadata routines are exact byte
sequences. Other routines, memory requests, coding, clearing, actuator
control, security access, ECU reset, and programming requests receive a
negative response without reaching the vehicle backend.

When an ENET gateway does not answer discovery, the service can use ID7's TASU
provider. It accepts only a syntactically valid VIN returned by the vehicle,
serializes requests through a local lease, filters replies by ECU and echoed
identifier, and does not synthesize a successful ECU response.

> [!WARNING]
> This boundary reduces risk but cannot make an arbitrary diagnostic client or
> vehicle safe. A permitted request reaches the real vehicle backend. Validate
> compatibility before relying on OBD Bridge for service or recovery work.

## Build and validation

From the repository root:

```sh
npm run test:obd
npm run build:obd
npm test
```

`npm run build:obd` cross-compiles a static Linux amd64 binary with
`-trimpath`, strips build metadata, updates its SHA-256, and records a source
fingerprint in `prebuilt/component.json`. Repository validation rejects stale
source or binary metadata.

The test suite covers protocol parsing, operation allowlisting, single-client
ownership, subnet and interface confinement, firewall lifecycle, Unix-socket
permissions, redacted exports, TASU request serialization, reply matching,
timeouts, cancellation, and shutdown cleanup. Loopback tests exercise ENET
discovery, keepalive exchange, and fault and data reads. BM3 compatibility varies
by vehicle and software version. BimmerLink still needs validation with a real
vehicle and phone.

## Diagnosing a failed connection on iDrive 7

The discovery counter increments only for the expected empty HSFZ vehicle
identification request from the hotspot subnet. It does not identify the
sending application. A reply requires a validated vehicle identity. A count
of received requests with zero answers can therefore result from a failed
vehicle route even when the hotspot input is working. The discovery write
failure counter identifies a separate local socket failure after resolution.
An accepted TCP connection confirms only the socket and peer checks.

`Check route` runs a bounded discovery and VIN probe independently of a phone.
With the TASU route, the service first checks the provider and then attempts
the existing read-only VIN request (functional address, DME, gateway).
The subscription counter records a completed registration call. The callback
counter records native deliveries before ownership, format and ECU filtering.
`REQUEST_PENDING` is a method verdict. An ECU response is evidenced separately
by the matching response metadata, including a negative response when supplied.
A functional VIN probe without a response is a timeout. A fresh failed route
check invalidates the previous identity cache.

The observed G21 trace establishes a failed route check and no discovery
reply. It does not establish why TASU was silent. The worker uses the same
known method, event, instance, and framing as the shared MGU diagnostic
client, but runs as a standalone process under the installed app UID. SDK
event delivery and vehicle routing require verification on the target.
TASU forwards selected UDS operations and provides a virtual HSFZ endpoint
with a validated VIN. Vendor adapter discovery and full gateway behavior
require separate implementations and evidence.

### Read-only vehicle verification

With the vehicle parked, run these on the MGU over your existing SSH access.
Use the interface and IPv4 shown by the app if they differ from the defaults.
Run the capture while attempting one connection from the phone, then stop it
with Ctrl+C. The capture command prints packet metadata without payloads.

```sh
systemctl status mgu-obd-adapter.service --no-pager
journalctl -u mgu-obd-adapter.service -n 100 --no-pager
ip -4 addr show dev wlan_bridge
ss -lnt '( sport = :6801 )'
ss -lnu '( sport = :6811 )'
nft list table ip filter | grep mgu-obd-adapter
tcpdump -ni wlan_bridge 'udp port 6811 or tcp port 6801'
curl --unix-socket /run/mgu-obd-adapter/control.sock http://localhost/status
curl --max-time 95 --unix-socket /run/mgu-obd-adapter/control.sock -X POST http://localhost/probe
curl --unix-socket /run/mgu-obd-adapter/control.sock http://localhost/export -o /tmp/hu-obd-diagnostics.json
```

Copy the redacted export over the same SSH connection. Check the route
without any connected tester first. Compare the subscription and callback
counts before and after the probe. Then connect MotorScope from the same
IPv4 subnet using the displayed MGU address and read faults from a known
ECU. Record the accepted TCP connection and delivered response separately.
For BimmerLink, record the exact adapter profile and whether it produces
discovery, TCP, and an actual diagnostic request. Wireless CarPlay and
cellular status do not establish an app's local network route. The xA Ultra
profile remains unverified with this generic endpoint.

### 1.1.3 discovery and retry behavior

A validated cached identity can answer consecutive discovery datagrams.
Invalid local datagrams are counted separately. A failed automatic route
check is cached for 30 seconds, so repeated discovery or TCP connection
attempts do not restart the full VIN sequence immediately. The manual route
check can retry immediately. TCP does not start a second TASU VIN sequence
when route resolution already failed.

A TASU reply timeout now records whether any native callback was received
while the request owned the stream. Callback presence alone does not prove
a matching ECU response. The G21 evidence still shows zero callbacks, so
the cause of missing native event delivery requires a vehicle or SDK trace.
