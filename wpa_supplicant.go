package wpa

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Wrap WPACtrl with commands for wpa_supplicant
type WPASupplicantCtrl struct {
	ctrl   Ctrl
	events chan WPASupplicantEvent
}

type WPASupplicantEvent interface {
	WPAString() string
}

type baseEvent struct {
	raw string
}

func (e *baseEvent) WPAString() string { return e.raw }

type OnConnectedEvent struct{ baseEvent }
type OnDisconnectedEvent struct {
	baseEvent
	reason string
}
type OnNotFoundEvent struct{ baseEvent }
type OnScanFailedEvent struct{ baseEvent }
type OnScanStartedEvent struct{ baseEvent }
type OnScanResultsEvent struct{ baseEvent }
type OnScanEvent struct{ baseEvent }

var dre = regexp.MustCompile("CTRL-EVENT-DISCONNECTED.*?reason=([0-9]+)")

func parseReason(msg string) string {
	found := dre.FindStringSubmatch(msg)
	if len(found) < 2 {
		return "0"
	}
	return found[1]
}

/* Reason codes (IEEE Std 802.11-2016, 9.4.1.7, Table 9-45) */
var commonReasonCodes = map[string]string{
	"2":  "invalid-auth",
	"3":  "sta-left-ess",
	"4":  "inactivity",
	"5":  "ap-overloaded",
	"6":  "class-2-nonauth",
	"7":  "class-3-nonassoc",
	"8":  "sta-left-bss",
	"9":  "not-authenticated-responder",
	"10": "bad-power-cap",
	"11": "bad-channels",
	"14": "mic-failure",
	"15": "four-way-handshake-timeout",
	"16": "group-key-handshake-timeout",
	"17": "four-way-handshake-mismatch",
	"18": "invalid-group-cipher",
	"19": "invalid-pairwise-cipher",
	"20": "invalid-akmp",
	"21": "unsupported-rsn",
	"22": "invalid-rsn",
	"23": "8021x-auth-failed",
	"24": "cipher-rejcted-due-to-policy",
	"32": "qos",
	"33": "qos-bandwidth",
	"34": "noisy-channel-cant-ack",
	"35": "outside-txop-limits",
	"36": "peer-leaving-bss",
	"37": "peer-rejects-mechanism",
	"38": "peer-mechanism-needs-setup",
	"39": "peer-timeout",
	"45": "peer-cipher-suite-not-supported",
}

func NewOnDisconnectedEvent(msg string) *OnDisconnectedEvent {
	reason := parseReason(msg)
	sreason := commonReasonCodes[reason]
	sreason = fmt.Sprintf("%s:%s", reason, sreason)

	return &OnDisconnectedEvent{baseEvent: baseEvent{msg}, reason: sreason}
}

func (e *OnDisconnectedEvent) Reason() string {
	return e.reason
}

// OnEvent is a catchall for events we aren't doing anything with (but might want to print)
type OnEvent struct{ baseEvent }

type Ctrl interface {
	Command(string) (string, error)
	OkCommand(string) error
	FailCommand(string) (string, error)
	Close()
	Attach() error
	Detach() error
	Unsolicited() <-chan string
}

func NewWPASupplicantCtrl(ctrl Ctrl, cmdTimeout time.Duration) *WPASupplicantCtrl {
	supCtrl := &WPASupplicantCtrl{
		ctrl:   ctrl,
		events: make(chan WPASupplicantEvent),
	}

	go func() {
		for msg := range ctrl.Unsolicited() {
			if strings.HasPrefix(msg, "CTRL-EVENT-CONNECTED") {
				supCtrl.events <- &OnConnectedEvent{baseEvent: baseEvent{msg}}
			} else if strings.HasPrefix(msg, "CTRL-EVENT-DISCONNECTED") {
				supCtrl.events <- NewOnDisconnectedEvent(msg)
			} else if strings.HasPrefix(msg, "CTRL-EVENT-NETWORK-NOT-FOUND") {
				supCtrl.events <- &OnNotFoundEvent{baseEvent: baseEvent{msg}}
			} else if strings.HasPrefix(msg, "CTRL-EVENT-SCAN-FAILED") {
				supCtrl.events <- &OnScanFailedEvent{baseEvent: baseEvent{msg}}
			} else if strings.HasPrefix(msg, "CTRL-EVENT-SCAN-STARTED") {
				supCtrl.events <- &OnScanStartedEvent{baseEvent: baseEvent{msg}}
			} else if strings.HasPrefix(msg, "CTRL-EVENT-SCAN-RESULTS") {
				supCtrl.events <- &OnScanResultsEvent{baseEvent: baseEvent{msg}}
			} else if strings.HasPrefix(msg, "CTRL-EVENT-BSS-ADDED") {
				supCtrl.events <- &OnScanEvent{baseEvent: baseEvent{msg}}
			} else {
				supCtrl.events <- &OnEvent{baseEvent: baseEvent{msg}}
			}
		}
	}()

	return supCtrl
}

func (c *WPASupplicantCtrl) Events() <-chan WPASupplicantEvent {
	return c.events
}

func (c *WPASupplicantCtrl) Close() {
	c.ctrl.Close()
}

func (c *WPASupplicantCtrl) Ctrl() Ctrl {
	return c.ctrl
}

// EVENT IMPLEMENTATIONS
// Currently not all implemented... adding them as needed.

func (c *WPASupplicantCtrl) AddNetwork() (string, error) {
	resp, err := c.ctrl.FailCommand("ADD_NETWORK")
	if err != nil {
		return "", err
	}

	return resp, nil

}

func (c *WPASupplicantCtrl) EnableNetwork(network string) error {
	return c.ctrl.OkCommand(fmt.Sprintf("ENABLE_NETWORK %s", network))
}

func (c *WPASupplicantCtrl) SetSSID(network string, ssid string) error {
	return c.ctrl.OkCommand(fmt.Sprintf("SET_NETWORK %s ssid \"%s\"", network, ssid))
}

func (c *WPASupplicantCtrl) SetPSK(network string, psk string) error {
	return c.ctrl.OkCommand(fmt.Sprintf("SET_NETWORK %s psk \"%s\"", network, psk))
}

type Network struct {
	ID   string
	SSID string
}

func (c *WPASupplicantCtrl) ListNetworks() ([]Network, error) {
	rsp, err := c.ctrl.FailCommand("LIST_NETWORKS")
	if err != nil {
		return nil, err
	}

	nets := strings.Split(rsp, "\n")[1:]

	result := make([]Network, len(nets))
	for i, net := range nets {
		f := strings.Split(net, "\t")
		result[i] = Network{
			ID:   f[0],
			SSID: f[1],
		}
	}
	return result, nil
}

func (c *WPASupplicantCtrl) RemoveNetwork(id string) error {
	return c.ctrl.OkCommand(fmt.Sprintf("REMOVE_NETWORK %s", id))
}
