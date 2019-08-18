package wpa

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrTimeout = errors.New("cmd timeout")

type Conn interface {
	Write([]byte) (int, error)
	Read([]byte) (int, error)
	Close() error
}

// WPACtrl maintains a command interface to wpa_supplicant or hostapd
// For more details: https://w1.fi/wpa_supplicant/devel/ctrl_iface_page.html
type WPACtrl struct {
	solicited   chan string
	unsolicited chan string

	ctx    context.Context
	cancel context.CancelFunc

	c Conn

	cmdTimeout time.Duration
}

// receiveLoop listens for datagrams on the control socket, and routes them to
// the appropriate channel.
// events, or "unsolicited" commands can occur at any time, and will be
// prefixed with a priority in angle brackets.
// solicited events only occur after a command has been sent, and have no
// priority prefix.
func (wc *WPACtrl) receiveLoop() {
	// individual messages arrive as a single datagrama, so a read should always contain
	// a full message.
	// Note: datagrams will be truncated if longer than this buffer size.
	// No obvious way to peek the next datagram size, but might be possible with syscalls.
	buf := make([]byte, 4096)
	defer close(wc.solicited)
	defer close(wc.unsolicited)
	for {
		n, err := wc.c.Read(buf)

		select {
		case <-wc.ctx.Done():
			// canceled, so error was probably that
			return
		default:
		}

		if err != nil {
			//logrus.WithError(err).Error()
			return
		}

		if buf[0] == byte('<') {
			// sanity check - should be <P> where P is a single digit priority
			if len(buf) < 3 || buf[2] != byte('>') {
				/*
					logrus.WithFields(logrus.Fields{
						"event": "invalid-solicited-msg",
						"msg":   string(buf[:n]),
					}).Error()
				*/
			} else {
				// we don't care about the priority prefix for now
				wc.unsolicited <- strings.TrimSpace(string(buf[3:n]))
			}
		} else {
			select {
			case wc.solicited <- strings.TrimSpace(string(buf[:n])):
			default:
				/*
					logrus.WithFields(logrus.Fields{
						"event": "unexpected-solicited-msg",
						"msg":   string(buf[:n]),
					}).Error()
				*/
			}
		}
	}
}

func NewWPACtrl(conn Conn, cmdTimeout time.Duration) *WPACtrl {

	ctx, cancel := context.WithCancel(context.Background())
	wc := &WPACtrl{
		c:           conn,
		solicited:   make(chan string, 1),
		unsolicited: make(chan string, 100),
		ctx:         ctx,
		cancel:      cancel,
		cmdTimeout:  cmdTimeout,
	}
	go wc.receiveLoop()
	return wc
}

func (wc *WPACtrl) Unsolicited() <-chan string {
	return wc.unsolicited
}

func (wc *WPACtrl) Command(cmd string) (string, error) {

	/*
		logrus.WithFields(logrus.Fields{
			"event": "wpa-cmd",
			"cmd":   cmd,
		}).Info()
	*/
	_, err := wc.c.Write([]byte(cmd))
	if err != nil {
		return "", fmt.Errorf("command error: %v", err)
	}

	select {
	case msg, ok := <-wc.solicited:
		if !ok {
			return "", errors.New("failed")
		}
		return msg, nil
	case <-time.After(wc.cmdTimeout):
		return "", ErrTimeout
	}

}

func (wc *WPACtrl) Close() {
	wc.Detach()
	wc.cancel()
	wc.c.Close()
}

// okCommand runs a wpa_ctrl command for which the normal
// response is just the string "OK"
// Any other response will be returned as an error.
// These are pretty common, hence this helper
func (c *WPACtrl) OkCommand(cmd string) error {
	rsp, err := c.Command(cmd)
	if err != nil {
		return err
	}
	if rsp != "OK" {
		return errors.New(rsp)
	}
	return nil
}

// failCommand runs a wpa_ctrl command which will spit out
// FAIL if it doesn't work
func (c *WPACtrl) FailCommand(cmd string) (string, error) {
	rsp, err := c.Command(cmd)
	if err != nil {
		return "", err
	}
	if rsp == "FAIL" {
		return "", errors.New(rsp)
	}
	return rsp, err
}

func (c *WPACtrl) Attach() error {
	return c.OkCommand("ATTACH")
}

func (c *WPACtrl) Detach() error {
	return c.OkCommand("DETACH")
}
