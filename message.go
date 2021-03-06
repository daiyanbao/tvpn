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

package tvpn

import (
	"net"
	"encoding/base64"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
)

type Message struct {
	To, From string
	Type     int
	Data     map[string]string
}

const (
	Init int = iota
	Join
	Quit
	Accept
	Deny
	Dhpub
	Tunnip
	Conninfo
	Reset
)

type messageError struct {
	message string
}

func (e messageError) Error() string {
	return e.message
}

const (
	initRE     string = `^INIT$`
	acceptRE          = `^ACCEPT$`
	denyRE            = `^DENY (?P<reason>.*)$`
	dhpubRE           = `^DHPUB (?P<i>[0-3]) (?P<x>[A-Za-z0-9+/=]+) (?P<y>[A-Za-z0-9+/=]+)$`
	tunnipRE          = `^TUNNIP (?P<ip>[0-9]{1,3}(?:\.[0-9]{1,3}){3})$`
	conninfoRE        = `^CONNINFO (?P<ip>[0-9]{1,3}(?:\.[0-9]{1,3}){3}) (?P<port>[0-9]+)$`
	resetRE           = `^RESET (?P<reason>.*)$`
)

func ParseMessage(message string) (*Message, error) {

	// BUG(Josh) Only compile regexps once
	init := regexp.MustCompile(initRE)
	accept := regexp.MustCompile(acceptRE)
	deny := regexp.MustCompile(denyRE)
	dhpub := regexp.MustCompile(dhpubRE)
	tunnip := regexp.MustCompile(tunnipRE)
	conninfo := regexp.MustCompile(conninfoRE)
	reset := regexp.MustCompile(resetRE)

	data := make(map[string]string)

	switch {
	case init.MatchString(message):
		return &Message{Type: Init, Data: data}, nil

	case accept.MatchString(message):
		return &Message{Type: Accept, Data: data}, nil

	case deny.MatchString(message):
		data["reason"] = deny.ReplaceAllString(message, "${reason}")
		return &Message{Type: Deny, Data: data}, nil

	case reset.MatchString(message):
		data["reason"] = reset.ReplaceAllString(message, "${reason}")
		return &Message{Type: Reset, Data: data}, nil

	case dhpub.MatchString(message):
		data["x"] = dhpub.ReplaceAllString(message, "${x}")
		data["y"] = dhpub.ReplaceAllString(message, "${y}")
		data["i"] = dhpub.ReplaceAllString(message, "${i}")
		return &Message{Type: Dhpub, Data: data}, nil

	case tunnip.MatchString(message):
		data["ip"] = tunnip.ReplaceAllString(message, "${ip}")
		return &Message{Type: Tunnip, Data: data}, nil

	case conninfo.MatchString(message):
		data["ip"] = conninfo.ReplaceAllString(message, "${ip}")
		data["port"] = conninfo.ReplaceAllString(message, "${port}")
		return &Message{Type: Conninfo, Data: data}, nil

	default:
		return nil, messageError{fmt.Sprintf("Failed to parse message: %s", message)}
	}

}

func (m Message) String() string {
	switch m.Type {
	case Init:
		return "INIT"
	case Deny:
		return fmt.Sprintf("DENY %s", m.Data["reason"])
	case Accept:
		return "ACCEPT"
	case Dhpub:
		return fmt.Sprintf("DHPUB %s %s %s", m.Data["i"], m.Data["x"], m.Data["y"])
	case Tunnip:
		return fmt.Sprintf("TUNNIP %s", m.Data["ip"])
	case Conninfo:
		return fmt.Sprintf("CONNINFO %s %s", m.Data["ip"], m.Data["port"])
	case Reset:
		return fmt.Sprintf("RESET %s", m.Data["reason"])
	}
	return ""
}

func (m Message) DhParams() (*big.Int, *big.Int, int, error) {
	if m.Type == Dhpub {
		xBytes, err := base64.StdEncoding.DecodeString(m.Data["x"])
		if err != nil {
			return nil, nil, 0, err
		}
		yBytes, err := base64.StdEncoding.DecodeString(m.Data["y"])
		if err != nil {
			return nil, nil, 0, err
		}
		x := &big.Int{}
		y := &big.Int{}
		x.SetBytes(xBytes)
		y.SetBytes(yBytes)
		i, _ := strconv.Atoi(m.Data["i"])
		return x, y, i, nil
	}
	return nil, nil, 0, nil
}

func (m Message) IPInfo() (net.IP,int) {
	if m.Type == Tunnip {
		return net.ParseIP(m.Data["ip"]),0
	}
	if m.Type == Conninfo {
		port,_ := strconv.Atoi(m.Data["port"])
		return net.ParseIP(m.Data["ip"]),port
	}
	return nil,0
}
