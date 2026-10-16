//go:build !(linux && arm)

package pocketbook

import "context"

// KeepWiFiOn connects to Wi-Fi on the reader. Off the reader, the host is assumed to be
// online already: there is nothing to do.
func KeepWiFiOn(ctx context.Context) (stop func(), err error) { return func() {}, nil }

// WiFiConnected reports whether the reader is connected to a network: off the reader, always.
func WiFiConnected() bool { return true }
