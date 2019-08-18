package wpatest

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
)

type network struct {
	id    int
	ssid  string
	psk   string
	flags string
}

type commandPair struct {
	cmd string
	rsp string
}

type ListenConn interface {
	ReadFrom([]byte) (int, net.Addr, error)
	Get(net.Addr) (Conn, error)
}

type Conn interface {
	Write([]byte) (int, error)
}

// WPAProcessMock attempts to roughly emulate the behavior of wpa_supplicant as far
// as state management goes, but doesn't interact with any hardware.
// There are some methods to fake responses and unsolicited messages that can be used
// to choreograph tests, which is useful for validating higher-level behavior.
type WPAProcessMock struct {
	Endpoint string
	t        *testing.T
	conn     ListenConn

	unsolConn Conn
	networks  []*network
	expect    *commandPair

	OnNetworkEnabled func(id int)
}

func NewWPAProcessMock(t *testing.T, conn ListenConn) *WPAProcessMock {
	w := &WPAProcessMock{
		conn: conn,
		t:    t,
	}
	go w.readLoop()
	return w
}

func (w *WPAProcessMock) getNetworkStr(idstr string) *network {
	id, err := strconv.Atoi(idstr)
	if err != nil {
		return nil
	}
	return w.getNetwork(id)
}

func (w *WPAProcessMock) getNetwork(id int) *network {
	if id < 0 || id > len(w.networks) {
		return nil
	}
	return w.networks[id]
}

// some fake implementation to help with higher level testing
func (w *WPAProcessMock) processMockCommand(cmd string, conn Conn) string {
	fields := strings.Split(cmd, " ")
	switch fields[0] {
	case "PING":
		return "PONG"
	case "ATTACH":
		w.unsolConn = conn
		return "OK"
	case "LIST_NETWORKS":
		lines := []string{"network id / ssid / bssid / flags"}
		for _, net := range w.networks {
			lines = append(lines, fmt.Sprintf("%d\t%s\t\t[%s]", net.id, net.ssid, net.flags))
		}
		return strings.Join(lines, "\n")

	case "ADD_NETWORK":
		id := len(w.networks)
		w.networks = append(w.networks, &network{id: id})
		return strconv.Itoa(id)
	case "REMOVE_NETWORK":
		if len(fields) < 2 {
			return "FAIL"
		}
		net := w.getNetworkStr(fields[1])
		if net == nil {
			return "FAIL"
		}
		if net.id == len(w.networks)-1 {
			w.networks = w.networks[:len(w.networks)-1]
		} else {
			w.networks[net.id] = nil
		}
		return "OK"
	case "ENABLE_NETWORK":
		if len(fields) < 2 {
			return "FAIL"
		}
		net := w.getNetworkStr(fields[1])
		if net == nil {
			return "FAIL"
		}
		net.flags = "CURRENT"
		go w.OnNetworkEnabled(net.id)
		return "OK"

	case "SET_NETWORK":
		if len(fields) < 4 {
			return "FAIL"
		}
		net := w.getNetworkStr(fields[1])
		if net == nil {
			return "FAIL"
		}
		switch fields[2] {
		case "ssid":
			net.ssid = strings.Trim(fields[3], "\"")
		case "psk":
			net.psk = strings.Trim(fields[3], "\"")
		default:
			return "FAIL"
		}
		return "OK"

	}
	return "UNKNOWN_COMMAND: " + fields[0]
}

func (w *WPAProcessMock) readLoop() {
	for {
		buf := make([]byte, 2048)
		n, outaddr, err := w.conn.ReadFrom(buf)
		if err != nil {
			fmt.Println(err)
			return
		}
		cmd := string(buf[:n])

		oc, err := w.conn.Get(outaddr)
		if err != nil {
			w.t.Error("couldn't get return conn", err)
			return
		}
		// If an expectation was set, then we are mocking the result,
		// so don't process the command, just send the rsp.
		var rsp string
		if w.expect != nil {
			if cmd != w.expect.cmd {
				w.t.Errorf("cmd %s is not %s", cmd, w.expect.cmd)
				return
			}
			rsp = w.expect.rsp
			w.expect = nil
		} else {
			rsp = w.processMockCommand(cmd, oc)
		}

		_, err = oc.Write([]byte(rsp))
		if err != nil {
			w.t.Error("response err", err)
			return
		}
	}
}

func (w *WPAProcessMock) SendUnsol(msg string) {
	n, err := w.unsolConn.Write([]byte(msg))
	if err != nil {
		w.t.Fatal(err)
	}
	if n != len(msg) {
		w.t.Fatal("incomplete send", n)
	}
}

func (w *WPAProcessMock) Expect(cmd string, rsp string) {
	if w.expect != nil {
		w.t.Fatal("already expecting", w.expect)
	}
	w.expect = &commandPair{
		cmd: cmd,
		rsp: rsp,
	}
}

func (w *WPAProcessMock) AnnounceConnected(id int) {
	net := w.getNetwork(id)
	if net == nil {
		w.t.Fatal("announce missing network", id)
	}
	w.SendUnsol(fmt.Sprintf("<2>CTRL-EVENT-CONNECTED - Connection to 00:1a:dd:18:a4:25 completed [id=%d id_str=]", id))
}

func (w *WPAProcessMock) AnnounceDisconnected(id int) {
	net := w.getNetwork(id)
	if net == nil {
		w.t.Fatal("announce missing network", id)
	}
	w.SendUnsol("<2>CTRL-EVENT-DISCONNECTED bssid=00:1a:dd:18:a4:25 reason=3 locally_generated=1")
}
