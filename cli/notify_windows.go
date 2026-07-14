//go:build windows

package main

import (
	"context"
	"fmt"
	"html"
	"os/exec"
	"strings"
	"time"
)

func sendNativeNotification(title string, body string, sound bool) error {
	escapeXML := func(value string) string { return html.EscapeString(value) }
	escapePowerShell := func(value string) string { return strings.ReplaceAll(value, "'", "''") }
	audio := "<audio silent='true'/>"
	if sound {
		audio = "<audio src='ms-winsoundevent:Notification.IM'/>"
	}
	toastXML := fmt.Sprintf("<toast>%s<visual><binding template='ToastGeneric'><text>%s</text><text>%s</text></binding></visual></toast>", audio, escapeXML(title), escapeXML(body))
	script := fmt.Sprintf(`$xml = New-Object Windows.Data.Xml.Dom.XmlDocument; $xml.LoadXml('%s'); [Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('Cliks').Show([Windows.UI.Notifications.ToastNotification]::new($xml))`, escapePowerShell(toastXML))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "powershell", "-NoProfile", "-NonInteractive", "-Command", script).Run()
}
