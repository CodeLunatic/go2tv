package devices

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/alexballas/go-ssdp"
	"golang.org/x/net/ipv4"
	"log"
	"net"
	"net/http"
)

const (
	networkType = "udp"
	groupAddr   = "239.255.255.250"
	port        = 1900
)

// =====================Init socket====================
// 使用一种比较生草的方式实现ssdp，目的是解决搜索不到本机的ssdp设备（服务）

var (
	socket    *net.UDPConn
	deviceMap map[string]Service
)

func init() {
	// 创建MulticastSocket并加入多播组，会占用1900端口
	var err error
	socket, err = joinMulticastGroup()
	if err != nil {
		log.Println("Error joining multicast group:", err)
		return // 一般1900被占用会出错
	}

	// 用于存储设备
	deviceMap = make(map[string]Service)

	go func() {
		// 接收响应
		receiveData := make([]byte, 1024)
		for {
			// 检查上下文的状态
			n, addr, err := socket.ReadFrom(receiveData)
			if err != nil {
				log.Println("Error receiving response:", err)
			}

			srv, err := parseService(addr, receiveData[:n])
			if err != nil {
				log.Printf("invalid search response from %s: %s\n", addr.String(), err)
			}
			if srv.Type != "" {
				deviceMap[srv.USN] = *srv
			}
			receiveData = make([]byte, 1024) // 清空接收缓冲区
		}
	}()
}

// AnotherSearch 只是用来辅助进行搜索，不需要做错误处理
func AnotherSearch(searchType string, waitSec int) []Service {

	if socket == nil {
		return []Service{}
	}

	// 此实现方式下发和不发消息都一个结果
	msg := buildSearch(searchType, waitSec)
	if err := sendMulticastMessage(socket, msg); err != nil {
		log.Println("Error sending M-SEARCH request:", err)
		return []Service{}
	}

	list := make([]Service, 0, len(deviceMap))
	for _, service := range deviceMap {
		list = append(list, service)
	}
	return list
}

func Search(searchType string, waitSec int, localAddr string) ([]interface{}, error) {

	deviceList1, err := ssdp.Search(searchType, waitSec, localAddr)
	if err != nil {
		return nil, err
	}

	results := make([]interface{}, 0, len(deviceList1)+len(deviceMap))
	for _, device := range deviceList1 {
		results = append(results, device)
	}

	deviceList2 := AnotherSearch(searchType, waitSec)
	for _, device := range deviceList2 {
		results = append(results, device)
	}
	return results, nil
}

// 把所有网卡（不支持多播的会跳过）生草的加入多播组
func joinMulticastGroup() (*net.UDPConn, error) {

	localAddr, err := net.ResolveUDPAddr(networkType, fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP(networkType, localAddr)
	if err != nil {
		return nil, err
	}

	groupIP := net.ParseIP(groupAddr)

	// 把所有可以组播的网卡加入组播中
	p := ipv4.NewPacketConn(conn)
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, ifi := range interfaces {
		if err := p.JoinGroup(&ifi, &net.UDPAddr{IP: groupIP}); err != nil {
			// 生草
			continue
		}
	}

	return conn, nil
}

// 发送多播消息，在这种方式下，发与不发都一样
func sendMulticastMessage(conn *net.UDPConn, data []byte) error {
	groupIP := net.ParseIP(groupAddr)
	sendToAddr := &net.UDPAddr{IP: groupIP, Port: port}

	if _, err := conn.WriteTo(data, sendToAddr); err != nil {
		return err
	}

	return nil
}

// =====================Tool Methods====================

func buildSearch(searchType string, waitSec int) []byte {
	b := new(bytes.Buffer)
	b.WriteString("M-SEARCH * HTTP/1.1\r\n")
	_, _ = fmt.Fprintf(b, "HOST: %s:%d\r\n", groupAddr, port)
	_, _ = fmt.Fprintf(b, "MAN: %q\r\n", "ssdp:discover")
	_, _ = fmt.Fprintf(b, "MX: %d\r\n", waitSec)
	_, _ = fmt.Fprintf(b, "ST: %s\r\n", searchType)
	b.WriteString("\r\n")
	return b.Bytes()
}

var (
	errWithoutHTTPPrefix = errors.New("without HTTP prefix")
)

var endOfHeader = []byte{'\r', '\n', '\r', '\n'}

func parseService(addr net.Addr, data []byte) (*Service, error) {
	lines := bytes.Split(data, []byte("\r\n"))

	if len(lines) > 0 {
		lines[0] = []byte("HTTP/1.1 200 OK")
	}

	data = bytes.Join(lines, []byte("\r\n"))

	if !bytes.HasPrefix(data, []byte("HTTP")) {
		return nil, errWithoutHTTPPrefix
	}
	// Complement newlines on tail of header for buggy SSDP responses.
	if !bytes.HasSuffix(data, endOfHeader) {
		// why we should't use append() for this purpose:
		// https://play.golang.org/p/IM1pONW9lqm
		data = bytes.Join([][]byte{data, endOfHeader}, nil)
	}
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(data)), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return &Service{
		From:      addr.String(),
		Type:      resp.Header.Get("NT"),
		USN:       resp.Header.Get("USN"),
		Location:  resp.Header.Get("LOCATION"),
		Server:    resp.Header.Get("SERVER"),
		rawHeader: resp.Header,
	}, nil
}
