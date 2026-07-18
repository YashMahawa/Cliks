//go:build linux

package main

import "testing"

func TestNotificationBusListRequiresProviderOwner(t *testing.T) {
	ready := "org.freedesktop.Notifications 97074 qs yash :1.886 user@1000.service - -\n"
	if !notificationBusListHasProvider(ready) {
		t.Fatal("active notification provider was not detected")
	}
	missing := "org.freedesktop.DBus 741 dbus-broker yash - user@1000.service - -\n"
	if notificationBusListHasProvider(missing) {
		t.Fatal("plain D-Bus broker was mistaken for a notification provider")
	}
}
