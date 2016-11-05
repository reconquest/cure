package main

import (
	"encoding/json"
	"net"
	"strconv"

	"github.com/kovetskiy/lorg"
	"github.com/reconquest/ser-go"
)

var (
	packetMaxSize   = 1024
	packetSignature = []byte{'C', 'U', 'R', 'E', 0}
)

type Peer struct {
	network    *net.IPNet
	port       int
	connection net.PacketConn
	address    *net.UDPAddr

	broadcastAddress *net.UDPAddr

	log lorg.Logger
}

func NewPeer(network *net.IPNet, port int, logger lorg.Logger) *Peer {
	return &Peer{
		network: network,
		port:    port,
		log:     logger,
	}
}

func (peer *Peer) connect() error {
	proto := "udp4"
	if peer.network.IP.To4() == nil {
		proto = "udp6"
	}

	address := &net.UDPAddr{
		IP:   peer.network.IP,
		Port: peer.port,
	}

	peer.log.Infof("listen %s", address)

	connection, err := net.ListenPacket(proto, address.String())
	if err != nil {
		return ser.Errorf(
			err, "can't listen: %s (%s)",
			address.String(), proto,
		)
	}

	peer.connection = connection
	peer.address = address
	peer.broadcastAddress = &net.UDPAddr{
		IP:   getBroadcastIP(peer.network),
		Port: peer.port,
	}

	peer.log.Infof("connection has been established")

	return nil
}

func (peer *Peer) observe() {
	assert(
		peer.connection != nil,
		"observing network without connection",
	)

	for {
		remote, packet, err := peer.read()
		if err != nil {
			peer.log.Error(
				ser.Errorf(err, "unable to read packet"),
			)

			continue
		}

		go peer.handle(remote, packet)
	}
}

func (peer *Peer) heartbeat() {
	assert(
		peer.connection != nil,
		"heartbeat without network connection",
	)

	packet := PacketPresence{
		Network: peer.network.String(),
	}

	data, err := encodePacket(packet)
	if err != nil {
		peer.log.Error(
			ser.Errorf(err, "can't encode packet: %#v", packet),
		)

		return
	}

	peer.broadcast(data)
}

func (peer *Peer) read() (net.Addr, []byte, error) {
	packet := make([]byte, packetMaxSize)

	size, remote, err := peer.connection.ReadFrom(packet)
	if err != nil {
		return nil, nil, err
	}

	return remote, packet[:size], nil
}

func (peer *Peer) handle(remote net.Addr, packet []byte) error {
	peer.log.Debugf("remote: %s; packet: %s", remote, packet)

	if peer.address.String() == remote.String() {
		peer.log.Debugf("skipping owned packet")
	}

	return nil
}

func (peer *Peer) broadcast(packet []byte) error {
	_, err := peer.connection.WriteTo(packet, peer.broadcastAddress)
	return err
}

func encodePacket(data interface{}) ([]byte, error) {
	packet, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	packet = signPacket(packet)

	return packet, nil
}

func signPacket(packet []byte) []byte {
	data := []byte{}

	data = append(data, packetSignature...)
	data = append(data, []byte(strconv.Itoa(len(packet)))...)
	data = append(data, byte(0))
	data = append(data, packet...)

	return data
}
