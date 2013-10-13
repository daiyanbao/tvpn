package tvpn

import (
	"github.com/Pursuit92/LeveledLogger/log"
	"encoding/base64"
	"fmt"
	"math/big"
	"net"
	"tvpn/dh"
)

type ConState struct {
	State        int
	Name         string
	Params       []dh.Params
	Key          []*big.Int
	IP           net.IP
	Port		 int
	Friend, Init bool
}

const (
	NoneState int = iota
	InitState
	DHNeg
	TunNeg
	ConNeg
	Connected
	DeleteMe
)

func (st *ConState) Input(mes Message, t TVPN) {
	switch st.State {
	case NoneState:
		st.noneState(mes, t.Sig)
	case InitState:
		st.initState(mes, t.Sig)
	case DHNeg:
		st.dhnegState(mes, t.Sig, t.Alloc)
	case TunNeg:
		st.tunnegState(mes, t.Sig,t.Stun, t.Alloc)
	case ConNeg:
		st.connegState(mes, t.Sig,t.VPN)
	case Connected:
		st.connectedState(mes, t.Sig)
	default:
	}
}

func (st *ConState) Reset(sig SigBackend,reason string) {
	st.State = NoneState
	st.Params = nil
	st.Key = nil
	st.IP = nil
	st.Port = 0
	if reason != "" {
		log.Out.Printf(3,"Conversation with %s reset. Reason: %s\n",st.Name,reason)
	}
	st.InitState(st.Name,st.Friend,st.Init,sig)
}

func (st *ConState) InitState(name string,friend,init bool,sig SigBackend) {
	st.Name = name
	st.Friend = friend
	st.Init = init
	if init {
		sig.SendMessage(Message{Type: Init, To: name})
		st.State = InitState
	} else {
		st.State = NoneState
	}
}

// NoneState is the state in which we wait for an Init
// Next state is DHNeg after a valid Init
func (st *ConState) noneState(mes Message, sig SigBackend) {
	switch mes.Type {
	case Init:
		if st.Friend {
			sig.SendMessage(Message{Type: Accept, To: st.Name})
			st.Params = make([]dh.Params, 4)
			st.Key = make([]*big.Int, 4)
			for i := 0; i < 4; i++ {
				st.Params[i] = dh.GenParams()
				sig.SendMessage(Message{Type: Dhpub, Data: map[string]string{
					"i": fmt.Sprintf("%d", i),
					"x": base64.StdEncoding.EncodeToString(st.Params[i].X.Bytes()),
					"y": base64.StdEncoding.EncodeToString(st.Params[i].Y.Bytes()),
				}})
			}
			st.State = DHNeg
		} else {
			sig.SendMessage(Message{Type: Deny, Data: map[string]string{"reason": "Not Authorized"}})
			st.State = DeleteMe
		}
	default:
		sig.SendMessage(Message{Type: Reset, Data: map[string]string{
			"reason": "Invalid state: None"}})
			st.Reset(sig,"")
	}
}

// Init state is after Init is sent and before Accept is received
// Next state is DHNeg
func (st *ConState) initState(mes Message, sig SigBackend) {
	switch mes.Type {
	case Accept:
		st.Params = make([]dh.Params, 4)
		st.Key = make([]*big.Int, 4)
		for i := 0; i < 4; i++ {
			st.Params[i] = dh.GenParams()
			sig.SendMessage(Message{Type: Dhpub, Data: map[string]string{
				"i": fmt.Sprintf("%d", i),
				"x": base64.StdEncoding.EncodeToString(st.Params[i].X.Bytes()),
				"y": base64.StdEncoding.EncodeToString(st.Params[i].Y.Bytes()),
			}})
		}
		st.State = DHNeg
	default:
		sig.SendMessage(Message{Type: Reset, Data: map[string]string{
			"reason": "Invalid state: Init"}})
			st.Reset(sig,"")
	}
}

func (st *ConState) dhnegState(mes Message, sig SigBackend, alloc IPManager) {
	switch mes.Type {
	case Dhpub:
		x, y, i, err := mes.DhParams()
		if err != nil {
			sig.SendMessage(Message{Type: Reset, Data: map[string]string{
				"reason": "Invalid DH Params",
			}})
			st.Reset(sig,"")
			return
		}
		st.Key[i] = dh.GenMutSecret(st.Params[i], dh.Params{X: x, Y: y})
		for _, v := range st.Key {
			if v == nil {
				// end state change - still need more keys
				return
			}
		}
		st.IP = alloc.Request(nil)
		sig.SendMessage(Message{Type: Tunnip, To: st.Name, Data: map[string]string{"ip": st.IP.String()}})
		st.State = TunNeg
	default:
		sig.SendMessage(Message{Type: Reset, Data: map[string]string{
			"reason": "Invalid state: DHNeg"}})
			st.Reset(sig,"")

	}
}

func (st *ConState) tunnegState(mes Message, sig SigBackend, stun StunBackend, alloc IPManager) {
	switch mes.Type {
	case Tunnip:
		ip,_ := mes.IPInfo()
		if isGreater(ip,st.IP) {
			alloc.Release(st.IP)
			st.IP = ip
		}
		st.Port = rgen.Int() % (65536 - 49152) + 49152
		ip,port,err := stun.DiscoverExt(st.Port)
		if err != nil {
			sig.SendMessage(Message{Type: Reset, Data: map[string]string{
				"reason": "Failed to discover external connection info"}})
			st.Reset(sig,"")
			return
		}
		sig.SendMessage(Message{Type: Conninfo, To: st.Name, Data: map[string]string{
			"port": fmt.Sprintf("%d",port),
			"ip": ip.String(),
		}})
		st.State = ConNeg
	default:
		sig.SendMessage(Message{Type: Reset, Data: map[string]string{
			"reason": "Invalid state: DHNeg"}})
			st.Reset(sig,"")
	}

}

func (st *ConState) connegState(mes Message,sig SigBackend, vpn VPNBackend) {
	switch mes.Type {
	case Conninfo:
		ip,port := mes.IPInfo()
		vpn.Connect(ip.String(),st.IP.String(),port,st.Port,st.Key,st.Init)
		st.State = Connected
	default:
		sig.SendMessage(Message{Type: Reset, Data: map[string]string{
			"reason": "Invalid state: DHNeg"}})
			st.Reset(sig,"")
	}
}


func (st *ConState) connectedState(mes Message, sig SigBackend) {
}
