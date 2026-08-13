package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/metacubex/mihomo/adapter"
	"github.com/metacubex/mihomo/constant"
	"gopkg.in/yaml.v3"
)

type nodePool struct {
	nodes []constant.Proxy
	index atomic.Int64
}

func loadNodePool(path string) (*nodePool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	pool := &nodePool{}
	for _, mapping := range doc.Proxies {
		if name, ok := mapping["name"].(string); !ok || name == "" {
			continue
		}
		p, err := adapter.ParseProxy(mapping)
		if err != nil {
			continue
		}
		pool.nodes = append(pool.nodes, p)
	}
	if len(pool.nodes) == 0 {
		return nil, fmt.Errorf("节点池为空")
	}
	return pool, nil
}

func (p *nodePool) Dial(ctx context.Context, metadata *constant.Metadata) (net.Conn, error) {
	n := len(p.nodes)
	if n == 0 {
		return nil, fmt.Errorf("无可用节点")
	}
	start := p.index.Add(1)
	var lastErr error
	for i := 0; i < n; i++ {
		node := p.nodes[(int(start)+i)%n]
		// 每个节点单独超时，避免坏节点阻塞整个请求
		dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		conn, err := node.DialContext(dialCtx, metadata)
		cancel()
		if err != nil {
			lastErr = err
			continue
		}
		return conn, nil
	}
	return nil, lastErr
}

func main() {
	// 单节点模式: 从环境变量解析
	pool, err := buildPool()
	if err != nil {
		fmt.Printf("初始化代理池失败: %v\n", err)
		os.Exit(1)
	}

	listenAddr := os.Getenv("PROXY_LISTEN")
	if listenAddr == "" {
		listenAddr = "127.0.0.1:7890"
	}
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Printf("监听失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("代理已启动: %s (节点数=%d)\n", listenAddr, len(pool.nodes))
	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go handleConn(conn, pool)
	}
}

func buildPool() (*nodePool, error) {
	// 优先节点池文件模式
	if poolFile := os.Getenv("PROXY_POOL_FILE"); poolFile != "" {
		pool, err := loadNodePool(poolFile)
		if err != nil {
			return nil, fmt.Errorf("加载节点池 %s 失败: %w", poolFile, err)
		}
		return pool, nil
	}
	// 单节点模式
	m := parseMapping()
	if len(m) == 0 {
		return nil, fmt.Errorf("未配置任何节点(PROXY_POOL_FILE或PROXY_*环境变量)")
	}
	p, err := adapter.ParseProxy(m)
	if err != nil {
		return nil, err
	}
	return &nodePool{nodes: []constant.Proxy{p}}, nil
}

func parseMapping() map[string]any {
	m := map[string]any{}
	if t := os.Getenv("PROXY_TYPE"); t != "" {
		m["type"] = t
	}
	for _, kv := range os.Environ() {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "PROXY_NAME":
			m["name"] = parts[1]
		case "PROXY_SERVER":
			m["server"] = parts[1]
		case "PROXY_PORT":
			if p, err := strconv.Atoi(parts[1]); err == nil {
				m["port"] = p
			}
		case "PROXY_UUID":
			m["uuid"] = parts[1]
		case "PROXY_NETWORK":
			m["network"] = parts[1]
		case "PROXY_FLOW":
			m["flow"] = parts[1]
		case "PROXY_SERVERNAME":
			m["servername"] = parts[1]
		case "PROXY_FINGERPRINT":
			m["client-fingerprint"] = parts[1]
		case "PROXY_PUBKEY":
			m["reality-opts"] = map[string]any{"public-key": parts[1], "short-id": os.Getenv("PROXY_SHORTID")}
		case "PROXY_PASSWORD":
			m["password"] = parts[1]
		case "PROXY_USERNAME":
			m["username"] = parts[1]
		case "PROXY_SNI":
			m["sni"] = parts[1]
		case "PROXY_TLS":
			m["tls"] = parts[1] == "true"
		case "PROXY_UDP":
			m["udp"] = parts[1] == "true"
		}
	}
	return m
}

func handleConn(client net.Conn, pool *nodePool) {
	defer client.Close()
	// 读取前2字节判断协议类型
	header := make([]byte, 2)
	if _, err := io.ReadFull(client, header); err != nil {
		return
	}

	// HTTP CONNECT 代理
	if header[0] == 'C' && header[1] == 'O' {
		// 读取完整 CONNECT 行
		buf := make([]byte, 0, 256)
		buf = append(buf, header...)
		tmp := make([]byte, 1)
		for {
			if _, err := io.ReadFull(client, tmp); err != nil {
				return
			}
			buf = append(buf, tmp[0])
			if len(buf) >= 4 && string(buf[len(buf)-4:]) == "\r\n\r\n" {
				break
			}
			if len(buf) > 4096 {
				return
			}
		}
		reqLine := string(buf)

		// 解析 CONNECT host:port HTTP/1.1
		var hostport string
		if n, _ := fmt.Sscanf(reqLine, "CONNECT %s", &hostport); n != 1 {
			return
		}
		if !strings.Contains(hostport, ":") {
			return
		}
		host, portStr, err := net.SplitHostPort(hostport)
		if err != nil {
			return
		}
		var u16Port uint16
		if p, err := strconv.Atoi(portStr); err == nil {
			u16Port = uint16(p)
		}

		ctx := context.Background()
		remote, err := pool.Dial(ctx, &constant.Metadata{
			Host:    host,
			DstPort: u16Port,
		})
		if err != nil {
			client.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
			return
		}
		defer remote.Close()
		client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

		// 双向转发
		done := make(chan struct{}, 2)
		go func() {
			io.Copy(remote, client)
			closeWrite(remote)
			done <- struct{}{}
		}()
		go func() {
			io.Copy(client, remote)
			closeWrite(client)
			done <- struct{}{}
		}()
		<-done
		<-done
		return
	}

	// SOCKS5 握手
	if header[0] != 0x05 {
		return
	}
	nMethods := int(header[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(client, methods); err != nil {
		return
	}
	client.Write([]byte{0x05, 0x00})

	// 读取请求
	reqHeader := make([]byte, 4)
	if _, err := io.ReadFull(client, reqHeader); err != nil {
		return
	}
	if reqHeader[0] != 0x05 {
		return
	}
	atype := reqHeader[3]
	var host string
	switch atype {
	case 0x01: // IPv4
		ip := make([]byte, 4)
		io.ReadFull(client, ip)
		host = net.IP(ip).String()
	case 0x03: // 域名
		l := make([]byte, 1)
		io.ReadFull(client, l)
		name := make([]byte, int(l[0]))
		io.ReadFull(client, name)
		host = string(name)
	case 0x04: // IPv6
		ip := make([]byte, 16)
		io.ReadFull(client, ip)
		host = net.IP(ip).String()
	default:
		return
	}
	portBytes := make([]byte, 2)
	io.ReadFull(client, portBytes)
	port := binary.BigEndian.Uint16(portBytes)

	// 通过节点池建立连接
	ctx := context.Background()
	remote, err := pool.Dial(ctx, &constant.Metadata{
		Host:    host,
		DstPort: port,
	})
	if err != nil {
		client.Write([]byte{0x05, 0x01, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer remote.Close()
	client.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0})

	// 双向转发
	done := make(chan struct{}, 2)
	go func() {
		io.Copy(remote, client)
		closeWrite(remote)
		done <- struct{}{}
	}()
	go func() {
		io.Copy(client, remote)
		closeWrite(client)
		done <- struct{}{}
	}()
	<-done
	<-done
}

func closeWrite(c net.Conn) {
	type closeWriter interface {
		CloseWrite() error
	}
	if cw, ok := c.(closeWriter); ok {
		cw.CloseWrite()
	}
}
