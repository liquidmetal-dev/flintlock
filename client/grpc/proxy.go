package grpc

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"

	gogrpc "google.golang.org/grpc"
)

// WithProxy is a grpc dial option that allows you to use an explicitly defined proxy server when making a gRPC
// request from a client.
// Adapted from: https://github.com/Axway/agent-sdk/blob/2d0067a8aef85e012f5314566f22d0333bec09fb/pkg/watchmanager/proxy.go#L23
func WithProxy(proxyURL *url.URL) gogrpc.DialOption {
	dialer := newGRPCProxyDialer(proxyURL)

	return gogrpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		return dialer.dial(ctx, addr)
	})
}

type proxyDialer interface {
	dial(ctx context.Context, addr string) (net.Conn, error)
}

type grpcProxyDialer struct {
	proxyAddress string
}

func newGRPCProxyDialer(proxyURL *url.URL) proxyDialer {
	return &grpcProxyDialer{
		proxyAddress: proxyURL.Host,
	}
}

func (g *grpcProxyDialer) dial(ctx context.Context, addr string) (net.Conn, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", g.proxyAddress)
	if err != nil {
		return nil, err
	}

	err = g.proxyConnect(ctx, conn, addr)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

func (g *grpcProxyDialer) proxyConnect(ctx context.Context, conn net.Conn, targetAddr string) error {
	req := g.createConnectRequest(ctx, targetAddr)
	if err := req.Write(conn); err != nil {
		return err
	}

	r := bufio.NewReader(conn)
	resp, err := http.ReadResponse(r, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to connect proxy, status : %s", resp.Status)
	}
	return nil
}

func (g *grpcProxyDialer) createConnectRequest(ctx context.Context, targetAddress string) *http.Request {
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: targetAddress},
	}

	return req.WithContext(ctx)
}
