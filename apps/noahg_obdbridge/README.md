# OBD Bridge

OBD Bridge exposes a bounded ENET diagnostic connection through the car's
existing Wi-Fi hotspot. Version `1.1.3` includes the ID7 control app and the
Store-managed `mgu-obd-adapter` companion service.

The bridge supports the observed bootmod3 (BM3) diagnostic protocol.
The BimmerLink connection over ENET Wi-Fi is experimental. Discovery, the
HSFZ keepalive, fault reads, and data reads are covered by simulated gateway
tests. BimmerLink on a phone with a real vehicle still needs validation.

> [!WARNING]
> Vehicle diagnostics can wake control units and alter their active diagnostic
> sessions even when persistent writes are blocked. Use OBD Bridge only while
> the vehicle is stationary, keep a recoverable diagnostic path available, and
> do not treat client discovery as proof that every vehicle or operation is
> supported.

Phones and computers join the car hotspot to connect. OBD Bridge:

- binds discovery and diagnostic listeners only to the selected hotspot
  interface;
- accepts clients from the hotspot subnet and permits one active client;
- relays a real vehicle gateway when one answers discovery;
- can use ID7's TASU service as a virtual route after reading and validating
  the vehicle VIN;
- forwards bounded identification, fault-reading, metadata, session, and
  tester-present operations; and
- refuses coding, clearing, actuator control, security access, ECU reset, and
  programming requests before they reach an ECU.

The UI can enable or disable the service, check the vehicle route, and save a
redacted diagnostic report. Routine status and exports omit VINs, serial
numbers, and raw diagnostic payloads.

## Try BimmerLink over ENET Wi-Fi

1. Install or update OBD Bridge and its companion service in MGU Store. Enable
   the car hotspot and OBD Bridge while the vehicle is parked.
2. Join the car's Wi-Fi hotspot on the phone. If cold discovery times out, use
   **Check route** in OBD Bridge and retry in BimmerLink as soon as the route
   check succeeds. The service caches the vehicle identity for 30 seconds.
3. Select an ENET Wi-Fi adapter profile in BimmerLink, such as **MHD ENET Wi-Fi**
   if offered, then connect. On iOS, grant BimmerLink Local Network access.
4. Check whether BimmerLink identifies the vehicle and can read faults and live
   values. If a step fails, **Save diagnostics** in OBD Bridge exports the
   discovery counters and redacted diagnostic operation metadata.

Available reads depend on the vehicle, adapter profile, and diagnostic route.
The bridge uses HSFZ over IPv4. It blocks functions such as clearing faults,
battery registration, and actuator control.

## Build and test

Requirements:

- Node.js 24 or later
- Python 3.12 or later
- Go 1.23 or later

From the repository root:

```sh
npm test
npm run test:obd
npm run build:obd
npm run build:apps
```

The app package and Linux amd64 service are committed with source fingerprints
and checksums. Rebuild the affected artifact after changing the app, shared
kit, native service, or TASU worker.

See the [service documentation](../../native/hu-obd-adapter/README.md) for the
network boundary and permitted diagnostic operations.
