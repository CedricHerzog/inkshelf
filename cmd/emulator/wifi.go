package main

import (
	"context"
	"errors"
	"net"
	"sync/atomic"

	"github.com/CedricHerzog/inkshelf/internal/app"
	"github.com/CedricHerzog/inkshelf/internal/device/pocketbook"
)

// errNoWiFi is the error of a connection made while the emulated Wi-Fi is off.
var errNoWiFi = errors.New("emulated Wi-Fi is off")

// wifiOff is whether the emulated reader has no network: -no-wifi turns it on at start, and
// the network controls (the page's, or a script's net up|down) change it. The app's HTTP
// client and its Wi-Fi options read it from their own goroutines.
var wifiOff atomic.Bool

// setNetwork turns the emulated network on or off, and tells the app as the firmware would.
func setNetwork(emulator *pocketbook.Emulator, up bool) {
	wifiOff.Store(!up)
	if up {
		emulator.SendSystem(pocketbook.NetConnected)
	} else {
		emulator.SendSystem(pocketbook.NetDisconnected)
	}
}

// withEmulatedWiFi makes options see the emulated Wi-Fi: with it off, the reader is not
// connected, and connecting fails at once rather than after the reader's timeout, so that
// scripts don't wait for it.
func withEmulatedWiFi(options app.Options) app.Options {
	options.WiFiConnected = func() bool { return !wifiOff.Load() }
	options.ConnectWiFi = func(context.Context) (func(), error) {
		if wifiOff.Load() {
			return nil, errNoWiFi
		}
		return func() {}, nil
	}
	return options
}

// dialUnlessOff fails while the emulated Wi-Fi is off, and dials with dial otherwise.
func dialUnlessOff(dial func(ctx context.Context, network, address string) (net.Conn, error)) func(ctx context.Context, network, address string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		if wifiOff.Load() {
			return nil, errNoWiFi
		}
		return dial(ctx, network, address)
	}
}
