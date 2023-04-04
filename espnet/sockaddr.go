package espnet

import (
	"strings"

	"github.com/embeddedgo/espat"
)

const maxConns = 10 // keep in sync with ../receiver.go

func getSockAddrs(d *espat.Device) ([]string, error) {
	cmdColon := "+CIPSTATUS:"
	status, err := d.CmdStr(cmdColon[:len(cmdColon)-1])
	if err != nil {
		return nil, err
	}
	ret := make([]string, 0, maxConns)
	i := 0
	for {
		k := strings.Index(status[i:], cmdColon)
		if k < 0 {
			break
		}
		i += k + len(cmdColon)
		k = strings.IndexByte(status[i:], '\n')
		if k < 0 {
			break
		}
		if sa := status[i : i+k]; len(sa) >= 2 {
			ret = append(ret, sa)
		}
		i = k + 1
	}
	return ret, nil
}

type netAddr struct {
	net, str string
}

func (a *netAddr) Network() string { return a.net }
func (a *netAddr) String() string  { return a.str }

// The returned strings are newly allocated and does not refer to the sa.
func parseSockAddr(sa, sport string) (addr netAddr, port string, server bool) {
	i := strings.IndexByte(sa, ',')
	if i < 0 {
		return
	}
	proto, sa := sa[1:i-1], sa[i+1:]
	switch proto {
	case "TCP":
		proto = "tcp"
	case "UDP":
		proto = "udp"
	}
	i = strings.IndexByte(sa, ',')
	if i < 0 {
		return
	}
	aa, sa := sa[1:i-1], sa[i+1:]
	i = strings.IndexByte(sa, ',')
	if i < 0 {
		return
	}
	ap, sa := sa[0:i], sa[i+1:]
	i = strings.IndexByte(sa, ',')
	if i < 0 {
		return
	}
	p, sa := sa[0:i], sa[i+1:]
	if len(sa) != 1 {
		return
	}
	server = sa[0] == '1'
	if sport == "" {
		port = ":" + p
	} else {
		port = sport
	}
	addr.net = proto
	ipv6 := strings.IndexByte(aa, ':') >= 0
	n := len(aa) + 1 + len(ap)
	if ipv6 {
		n += 2
	}
	var sb strings.Builder
	sb.Grow(n)
	if ipv6 {
		sb.WriteByte('[')
	}
	sb.WriteString(aa)
	if ipv6 {
		sb.WriteByte('[')
	}
	sb.WriteByte(':')
	sb.WriteString(ap)
	addr.str = sb.String()
	addr.net = proto
	return
}
