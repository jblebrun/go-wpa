package wpa

import (
	"fmt"
	"testing"
	"time"

	"github.com/jblebrun/go-wpa/wpatest"
)

func NewTempConn(t *testing.T) (*wpatest.TestConn, *wpatest.TestConn) {
	lc, err := wpatest.NewTestListenConn()
	if err != nil {
		t.Fatal(err)
	}
	c, err := lc.Dial()
	if err != nil {
		t.Fatal(err)
	}
	return lc, c
}

func NewWPATest(t *testing.T) (*wpatest.WPAProcessMock, *WPACtrl) {
	lc, c := NewTempConn(t)
	mock := wpatest.NewWPAProcessMock(t, lc)

	ctrl := NewWPACtrl(c, time.Second)
	return mock, ctrl
}

func NewWPASupplicantTest(t *testing.T) (*wpatest.WPAProcessMock, *WPASupplicantCtrl) {
	lc, c := NewTempConn(t)
	mock := wpatest.NewWPAProcessMock(t, lc)

	bctrl := NewWPACtrl(c, 5*time.Second)

	ctrl := NewWPASupplicantCtrl(bctrl, time.Second)
	return mock, ctrl
}

func TestCommand(t *testing.T) {
	_, ctrl := NewWPATest(t)

	rsp, err := ctrl.Command("PING")
	if err != nil {
		t.Fatal(err)
	}
	if rsp != "PONG" {
		t.Fatal("rsp not ok: ", rsp)
	}
}

func TestCommandTimeout(t *testing.T) {
	_, c := NewTempConn(t)
	ctrl := NewWPACtrl(c, time.Microsecond)

	_, err := ctrl.Command("PING")
	if err != ErrTimeout {
		t.Fatal("expect timeout")
	}
}

func TestOkCommand(t *testing.T) {
	mock, ctrl := NewWPATest(t)
	mock.Expect("TEST_CMD", "OK")
	err := ctrl.OkCommand("TEST_CMD")
	if err != nil {
		t.Fatal(err)
	}
}

func TestFailCommand(t *testing.T) {
	mock, ctrl := NewWPATest(t)
	mock.Expect("TEST_BAD_CMD", "FAIL")
	_, err := ctrl.FailCommand("TEST_BAD_CMD")
	if err == nil || err.Error() != "FAIL" {
		fmt.Println("expect FAIL err, got: ", err)
	}
}

func TestAttach(t *testing.T) {
	_, ctrl := NewWPASupplicantTest(t)

	err := ctrl.Ctrl().Attach()
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddAndConfigureNetwork(t *testing.T) {
	_, ctrl := NewWPASupplicantTest(t)

	net := Network{
		ID:   "0",
		SSID: "foossid",
	}

	id, err := ctrl.AddNetwork()
	if err != nil {
		t.Fatal(err)
	}
	if id != "0" {
		t.Fatal("wrong resp", id)
	}

	if err := ctrl.SetSSID("0", "foossid"); err != nil {
		t.Fatal(err)
	}
	if err := ctrl.SetPSK("0", "foopsk"); err != nil {
		t.Fatal(err)
	}

	nets, err := ctrl.ListNetworks()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 1 {
		t.Fatal("wrong number of nets", len(nets))
	}
	if nets[0] != net {
		t.Fatal("wrong net 0", nets[0], net)
	}
	if err := ctrl.RemoveNetwork("0"); err != nil {
		t.Fatal(err)
	}
	nets, err = ctrl.ListNetworks()
	if err != nil {
		t.Fatal(err)
	}
	if len(nets) != 0 {
		t.Fatal("didnt remove", nets)
	}
}

func TestUnsol(t *testing.T) {
	mock, ctrl := NewWPATest(t)

	if err := ctrl.Attach(); err != nil {
		t.Fatal(err)
	}

	msg := "CTRL-EVENT-SOMETHING"
	mock.SendUnsol(fmt.Sprintf("<2>%s", msg))

	select {
	case rmsg := <-ctrl.Unsolicited():
		if rmsg != msg {
			t.Fatal("expect", msg, "got", rmsg)
		}
	case <-time.After(time.Second):
		t.Fatal("no msg")
	}
}

func TestWPASuppEvents(t *testing.T) {
	mock, ctrl := NewWPASupplicantTest(t)

	if err := ctrl.Ctrl().Attach(); err != nil {
		t.Fatal(err)
	}

	msg := "CTRL-EVENT-CONNECTED"
	mock.SendUnsol(fmt.Sprintf("<2>%s", msg))

	select {
	case evt := <-ctrl.Events():
		switch evt.(type) {
		case *OnConnectedEvent:
		default:
			t.Fatalf("wrong event %+v", evt)
		}
	case <-time.After(time.Second):
		t.Fatal("no msg")
	}
}

type fakeNet Network

func (f fakeNet) String() string {
	return fmt.Sprintf("%s\t%s\t\t[]", f.ID, f.SSID)
}

func TestDisconMsg(t *testing.T) {

	msg := "CTRL-EVENT-DISCONNECTED bssid=00:1a:dd:18:f2:45 reason=2"

	de := NewOnDisconnectedEvent(msg)

	if de.Reason() != "2:invalid-auth" {
		t.Fatal("wrong reason", de)
	}
}

func TestNoDisconMsg(t *testing.T) {

	msg := "CTRL-EVENT-DISCONNECTED bssid=00:1a:dd:18:f2:45"

	de := NewOnDisconnectedEvent(msg)

	if de.Reason() != "0:" {
		t.Fatal("wrong reason", de)
	}
}

func TestBadDisconMsg(t *testing.T) {

	msg := "CTRL-EVENT-DISCONNECTED bssid=00:1a:dd:18:f2:45 reason=1235"

	de := NewOnDisconnectedEvent(msg)

	if de.Reason() != "1235:" {
		t.Fatal("wrong reason", de)
	}
}

func TestBadDisconMsg2(t *testing.T) {

	msg := "CTRL-EVENT-DISCONNECTED bssid=00:1a:dd:18:f2:45 reason=asdf4"

	de := NewOnDisconnectedEvent(msg)

	if de.Reason() != "0:" {
		t.Fatal("wrong reason", de)
	}
}
