/*
 *  TVPN: A Peer-to-Peer VPN solution for traversing NAT firewalls
 *  Copyright (C) 2013  Joshua Chase <jcjoshuachase@gmail.com>
 *
 *  This program is free software; you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation; either version 2 of the License, or
 *  (at your option) any later version.
 *
 *  This program is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License along
 *  with this program; if not, write to the Free Software Foundation, Inc.,
 *  51 Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
*/

package ovpn

import (
	"fmt"
	"net"
	"io"
	"os"
	"os/exec"
	"github.com/Pursuit92/tvpn"
	"github.com/Pursuit92/LeveledLogger/log"
)

type OVPNBackend struct {
	path,tmp string
}

type OVPNConn struct {
	Cmd *exec.Cmd
	outBuffer io.Reader
	errBuffer io.Reader
}

func (ovpn *OVPNBackend) Configure(conf tvpn.VPNConfig) {
	ovpn.tmp = conf["Tmp"]
	ovpn.path = conf["Path"]
}

func (ovpn *OVPNBackend) Connect(remoteip,tunIP net.IP,
	remoteport,localport int,
	key [][64]byte,
	dir bool,
	routes map[string]string) (tvpn.VPNConn,error) {


	var dirS string
	var localtun,remotetun net.IP
	localtun = make([]byte, 16)
	remotetun = make([]byte, 16)
	copy(localtun,tunIP)
	copy(remotetun,tunIP)
	if dir {
		dirS = "1"
		localtun[len(localtun)-1]++
		remotetun[len(remotetun)-1] += 2
	} else {
		dirS = "0"
		localtun[len(localtun)-1] += 2
		remotetun[len(remotetun)-1]++
	}

	keyfile := fmt.Sprintf("%s%s-%d.key",ovpn.tmp,remoteip.String(),remoteport)
	log.Out.Lprintf(2,"Creating key file %s\n",keyfile)
	keyhandle,e := os.Create(keyfile)
	if e != nil {
		log.Out.Lprintln(1,e)
		return nil,e
	}
	_,e = keyhandle.Write(EncodeOpenVPNKey(key))
	if e != nil {
		log.Fatal(e)
		return nil,e
	}
	keyhandle.Close()

	opts := append(ovpnOpts,
			"--remote", remoteip.String(),
			"--rport", fmt.Sprintf("%d",remoteport),
			"--lport", fmt.Sprintf("%d",localport),
			"--secret", keyfile, dirS,
			"--ifconfig", localtun.String(), "255.255.255.252")

	for r,m := range routes {
		opts = append(opts,"--route",r,m,remotetun.String())
	}

	cmd := exec.Command(ovpn.path, opts...)


	log.Out.Lprintf(2,"Running command: %s ",cmd.Path)
	for _,v := range cmd.Args {
		log.Out.Lprintf(2,"%s ",v)
	}
	log.Out.Lprint(2,"\n")
	out,e := cmd.StdoutPipe()

	if e != nil {
		log.Fatal(e)
	}
	err,e := cmd.StderrPipe()

	if e != nil {
		log.Fatal(e)
	}
	e = cmd.Start()
	if e != nil {
		log.Out.Lprintln(1,e)
		return nil, e
	}

	conn := &OVPNConn{Cmd: cmd}

	conn.outBuffer = out
	conn.errBuffer = err

	log.Out.Lprintf(2,"\nVPN Connected with pid %d\n",cmd.Process.Pid)
	return conn,nil
}

func (conn *OVPNConn) Disconnect() {
	log.Out.Lprintf(2,"Killing process with PID %d\n",conn.Cmd.Process.Pid)
	proc := conn.Cmd.Process
	proc.Kill()
}

func (conn OVPNConn) Connected() bool {
	return ! conn.Cmd.ProcessState.Exited()
}

func (conn OVPNConn) Log() (io.Reader,io.Reader) {
	return conn.outBuffer,conn.errBuffer
}

var ovpnOpts = []string{
	"--mode", "p2p",
	"--proto", "udp",
	"--dev", "tap",
	"--ping-exit", "30",
	"--ping", "10",
	"--suppress-timestamps",
}
