package esplnet

import (
	"errors"
	"strconv"
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
		i += k + 1
	}
	return ret, nil
}

type Addr struct {
	net      string
	hostPort string
}

func (a *Addr) Network() string { return a.net }
func (a *Addr) String() string  { return a.hostPort }

// The returned strings are newly allocated and does not refer to the sa.
func parseSockAddr(sa string) (net, local, remote string, server bool) {
	i := strings.IndexByte(sa, ',')
	if i < 0 {
		return
	}
	proto, sa := sa[1:i-1], sa[i+1:]
	switch proto {
	case "TCP":
		net = "tcp"
	case "UDP":
		net = "udp"
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
	local = ":" + p
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
	remote = sb.String()
	return
}

func splitHostPort(net, addr string) (proto, host, port string, pn int, err error) {
	var proto6 string
	switch net {
	case "tcp", "tcp6", "tcp4":
		proto6 = "TCPv6"
	case "udp", "udp6", "udp4":
		proto6 = "UDPv6"
	default:
		err = errors.New("unknown network ") // ended with space to match net
		return
	}
	if len(addr) == 0 {
		err = errors.New("empty address")
		return
	}
	proto = proto6
	nlc := net[len(net)-1]
	if nlc != '6' {
		proto = proto6[:3]
	}
	if nlc != '4' && addr[0] == '[' {
		i := strings.LastIndexByte(addr, ']')
		if i >= 0 && i+1 < len(addr) && addr[i+1] == ':' {
			host, port = addr[1:i], addr[i+2:]
			proto = proto6
		}
	}
	if host == "" {
		i := strings.LastIndexByte(addr, ':')
		if i < 0 {
			err = errors.New("missing port in address")
			return
		}
		host, port = addr[:i], addr[i+1:]
	}
	u, _ := strconv.ParseUint(port, 10, 16)
	pn = int(u)
	if pn == 0 {
		err = errors.New("unknown port")
	}
	return
}
