package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"

	"github.com/mikkeloscar/sshconfig"
)

type cmd struct {
	start bool
	host  *sshconfig.SSHHost
}

type TunnelManager struct {
	port int
	curr string
	proc *os.Process
	cmd  chan *cmd
	resp chan bool
	sync.Mutex
}

func (t *TunnelManager) Start(host *sshconfig.SSHHost) bool {
	t.Lock()
	t.cmd <- &cmd{true, host}
	defer t.Unlock()

	return <-t.resp
}

func (t *TunnelManager) Stop() bool {
	t.Lock()
	t.cmd <- &cmd{false, nil}
	defer t.Unlock()

	return <-t.resp
}

func (t *TunnelManager) run(port int) {
	var cmd *cmd
	var err error

	// set ssh tunnel port
	t.port = port

	for {
		cmd = <-t.cmd

		if cmd.start {
			err = t.start(cmd.host)
			if err != nil {
				// TODO log error?
				t.resp <- false
			} else {
				t.resp <- true
			}
		} else {
			err = t.stop()
			if err != nil {
				// TODO log error?
				t.resp <- false
			} else {
				t.resp <- true
			}
		}
	}
}

func (t *TunnelManager) start(host *sshconfig.SSHHost) error {
	// stop previous process if running
	if t.curr != host.Host[0] {
		t.stop()
	}

	// start ssh tunnel
	if t.curr != host.Host[0] || t.proc == nil {
		sshCmd := exec.Command("/usr/bin/ssh",
			"-D", fmt.Sprintf("%d", t.port),
			"-N", fmt.Sprintf("%s@%s", host.User, host.HostName),
			"-p", fmt.Sprintf("%d", host.Port))
		err := sshCmd.Start()
		if err != nil {
			return err
		}
		t.proc = sshCmd.Process
		t.curr = host.Host[0]
	}

	return nil
}

func (t *TunnelManager) stop() error {
	if t.proc != nil {
		err := t.proc.Kill()
		if err != nil {
			return err
		}

		_, err = t.proc.Wait()
		if err != nil {
			return err
		}

		if err != nil {
			return err
		}

		t.proc = nil
	}

	return nil
}
