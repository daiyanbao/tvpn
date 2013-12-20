package tvpn

import (
	"github.com/Pursuit92/LeveledLogger/log"
	"math/rand"
	"time"
	"fmt"
)

type TVPN struct {
	Name, Group string
	Friends     []string
	Sig         SigBackend
	Stun        StunBackend
	VPN			VPNBackend
	States      map[string]*ConState
	Alloc		IPManager
}

var rgen *rand.Rand
func init() {
	rgen = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func SetLogLevel(n int) {
	log.Out.SetLevel(n)
}

func SetLogPrefix(s string) {
	for i := 0; i < 10; i++ {
		log.Out.SetPrefix(i,s)
	}
}

func New(name, group string,
	friends []string,
	sig SigBackend,
	stun StunBackend,
	vpn VPNBackend, alloc IPManager) *TVPN {

	tvpnInstance := TVPN{
		Name:       name,
		Group:      group,
		Sig:  sig,
		Stun: stun,
		Friends: friends,
		VPN: vpn,
		Alloc: alloc,
	}
	tvpnInstance.States = make(map[string]*ConState)

	return &tvpnInstance
}

func (t TVPN) IsFriend(name string) bool {
	for _,v := range t.Friends {
		if v == name {
			return true
		}
	}
	return false
}


func (t *TVPN) Run() error {
	for {
		fmt.Printf("Waiting for message...\n")
		msg := t.Sig.RecvMessage()
		fmt.Printf("Got a message: %s\n",msg.String())
		switch msg.Type {
		case Init:
			friend := t.IsFriend(msg.From)
			fmt.Printf("Creating new state machine for %s\n",msg.From)
			t.States[msg.From] = NewState(msg.From,friend,false,*t)
			t.States[msg.From].Input(msg,*t)
		case Join:
			fmt.Printf("Received Join from %s!\n",msg.From)
			if t.IsFriend(msg.From) {
				t.States[msg.From] = NewState(msg.From,true,true,*t)
			}
			fmt.Println("Done with join!")

		case Quit:
			st,exists := t.States[msg.From]
			if exists {
				st.Cleanup(*t)
				delete(t.States,msg.From)
			}
		case Reset:
			t.States[msg.From].Reset(msg.Data["reason"],*t)
		default:
			st,exists := t.States[msg.From]
			if exists {
				st.Input(msg,*t)
			} else {
				// do stuff here
			}
		}
	}
}
