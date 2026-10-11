//go:build linux && arm

package pocketbook

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
	"time"

	"github.com/CedricHerzog/inkshelf/internal/timing"
)

// netagent is the firmware's network tool. libinkview's NetConnect runs it the same way: "net
// on" connects to a known network, and fills /etc/resolv.conf so that Go's own resolver finds
// the servers' addresses.
const netagent = "/ebrmain/bin/netagent"

// networkTimeout is how long KeepWiFiOn waits for Wi-Fi.
const networkTimeout = 40 * time.Second

// connectPollInterval is how often KeepWiFiOn checks whether Wi-Fi is connected.
const connectPollInterval = 250 * time.Millisecond

// KeepWiFiOn connects to a known Wi-Fi network if not connected yet, giving up when ctx is
// done. stop turns Wi-Fi back off if KeepWiFiOn turned it on, and does nothing otherwise.
func KeepWiFiOn(ctx context.Context) (stop func(), err error) {
	if connected() {
		return func() {}, nil
	}
	ctx, cancel := context.WithTimeout(ctx, networkTimeout)
	defer cancel()
	span := timing.Start(ctx, "connect Wi-Fi")
	output, err := exec.CommandContext(ctx, netagent, "net", "on").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("netagent net on: %w: %s", err, strings.TrimSpace(string(output)))
	}
	for !connected() {
		select {
		case <-ctx.Done():
			return nil, errors.New("no Wi-Fi connection")
		case <-time.After(connectPollInterval):
		}
	}
	log.Printf("Wi-Fi connected in %v", span.End())
	return func() {
		// It runs after KeepWiFiOn has returned and canceled ctx.
		if output, err := exec.CommandContext(context.Background(), netagent, "net", "off").CombinedOutput(); err != nil {
			log.Printf("netagent net off: %v: %s", err, strings.TrimSpace(string(output)))
		}
	}, nil
}

// connected reports whether an interface other than loopback is up with an IPv4 address.
func connected() bool {
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}
	for _, networkInterface := range interfaces {
		if networkInterface.Flags&net.FlagUp == 0 || networkInterface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := networkInterface.Addrs()
		if err != nil {
			// An interface whose addresses can't be read can't be used either.
			continue
		}
		for _, address := range addresses {
			if ipNet, ok := address.(*net.IPNet); ok && ipNet.IP.To4() != nil {
				return true
			}
		}
	}
	return false
}
