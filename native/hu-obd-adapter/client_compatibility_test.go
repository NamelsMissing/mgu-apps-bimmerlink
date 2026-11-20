package main

import (
	"bytes"
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestENETDiscoveryAndReadsThroughVirtualGateway(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	udp, e := net.ListenPacket("udp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer udp.Close()
	tcp, e := net.Listen("tcp4", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer tcp.Close()
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	identity, e := tasuIdentification("WBA12345678901234", tasuGatewayAddress, net.HardwareAddr{2, 0, 0, 0, 0, 1})
	if e != nil {
		t.Fatal(e)
	}
	s := newService(defaults())
	s.resolve = func(context.Context) (gateway, error) {
		return gateway{Backend: "tasu", Address: "tasu", Identification: identity}, nil
	}
	backend, bridge := net.Pipe()
	defer backend.Close()
	defer bridge.Close()
	s.open = func(context.Context, gateway) (vehicle, error) { return gatewaySession{bridge}, nil }
	go s.serveDiscovery(ctx, udp, network)
	go s.serveTCP(ctx, tcp, network)
	d := net.Dialer{Timeout: 2 * time.Second}
	probe, e := d.Dial("udp4", udp.LocalAddr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer probe.Close()
	probe.SetDeadline(time.Now().Add(2 * time.Second))
	if e = writeAll(probe, discoveryProbe); e != nil {
		t.Fatal(e)
	}
	buf := make([]byte, 128)
	if n, e := probe.Read(buf); e != nil || !bytes.Equal(buf[:n], identity) {
		t.Fatalf("unexpected discovery response: %x (%v)", buf[:n], e)
	}
	client, e := d.Dial("tcp4", tcp.Addr().String())
	if e != nil {
		t.Fatal(e)
	}
	defer client.Close()
	client.SetDeadline(time.Now().Add(2 * time.Second))
	backend.SetDeadline(time.Now().Add(2 * time.Second))
	expect := func(conn net.Conn, want []byte) {
		t.Helper()
		if got, e := readFrame(conn); e != nil || !bytes.Equal(got, want) {
			t.Fatalf("want %x, received %x (%v)", want, got, e)
		}
	}
	send := func(conn net.Conn, b []byte) {
		t.Helper()
		if e := writeAll(conn, b); e != nil {
			t.Fatal(e)
		}
	}
	alive := []byte{0xf4, tasuGatewayAddress}
	send(client, frame(0x12, alive))
	expect(client, frame(2, alive))
	send(client, frame(0x13, []byte{0, 0xf4}))
	clear := []byte{0xf4, 0x12, 0x14, 0xff, 0xff, 0xff}
	send(client, frame(1, clear))
	expect(client, frame(2, clear))
	expect(client, frame(1, []byte{0x12, 0xf4, 0x7f, 0x14, 0x11}))
	for _, pair := range [][2][]byte{
		{{0xf4, 0x12, 0x19, 2, 0xff}, {0x12, 0xf4, 0x59, 2, 0xff}},
		{{0xf4, 0x12, 0x22, 0x40, 0x98}, {0x12, 0xf4, 0x62, 0x40, 0x98, 1}},
	} {
		request, reply := frame(1, pair[0]), frame(1, pair[1])
		send(client, request)
		expect(backend, request)
		send(backend, frame(2, pair[0]))
		send(backend, reply)
		expect(client, frame(2, pair[0]))
		expect(client, reply)
	}
}

// This opt-in check loads the owner's installed Java transport without copying
// vendor code into the repository or accessing an account or physical vehicle.
func TestInstalledBimmerCodeENETTransport(t *testing.T) {
	app := os.Getenv("MGU_BIMMERCODE_APP")
	if app == "" {
		t.Skip("set MGU_BIMMERCODE_APP for the installed-client transport check")
	}
	java := os.Getenv("MGU_JAVA")
	harness := os.Getenv("MGU_BIMMERCODE_HARNESS")
	if java == "" || harness == "" {
		t.Fatal("Java and compiled loopback harness required")
	}
	udp, e := net.ListenPacket("udp4", "127.0.0.1:6811")
	if e != nil {
		t.Fatal(e)
	}
	defer udp.Close()
	tcp, e := net.Listen("tcp4", "127.0.0.1:6801")
	if e != nil {
		t.Fatal(e)
	}
	defer tcp.Close()
	_, network, _ := net.ParseCIDR("127.0.0.0/8")
	c := defaults()
	c.TASUFallback = false
	s := newService(c)
	s.resolve = func(context.Context) (gateway, error) {
		return gateway{Address: "127.0.0.2", Identification: testIdentification()}, nil
	}
	s.open = func(context.Context, gateway) (vehicle, error) {
		client, server := net.Pipe()
		go func() {
			defer server.Close()
			for {
				b, e := readFrame(server)
				if e != nil {
					return
				}
				if e = writeAll(server, frame(2, b[6:])); e != nil {
					return
				}
				var uds []byte
				switch b[8] {
				case 0x10:
					uds = []byte{0x50, 3, 0, 50, 1, 244}
				case 0x3e:
					uds = []byte{0x7e, 0}
				case 0x22:
					uds = append([]byte{0x62, 0xf1, 0x90}, []byte("WBA12345678901234")...)
				default:
					return
				}
				if writeAll(server, frame(1, append([]byte{b[7], b[6]}, uds...))) != nil {
					return
				}
			}
		}()
		return gatewaySession{client}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	go s.serveDiscovery(ctx, udp, network)
	go s.serveTCP(ctx, tcp, network)
	cp := harness + string(os.PathListSeparator) + filepath.Join(app, "*")
	if extra := os.Getenv("MGU_JAVA_EXTRA_CLASSPATH"); extra != "" {
		cp += string(os.PathListSeparator) + extra
	}
	cmd := exec.CommandContext(ctx, java, "-cp", cp, "BimmerCodeConnection", "127.0.0.1")
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("installed client: %v\n%s", e, out)
	}
	t.Log(string(out))
	if st := s.snapshot(); st.DiscoveryReplies != 1 || st.Requests != 3 {
		t.Fatal(st)
	}
}
