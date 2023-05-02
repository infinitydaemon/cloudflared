package tunnel

import (
	"fmt"

	"github.com/cloudflare/cloudflared/tunneldns"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/urfave/cli/v2"
)

const (
	proxyDNSPortKey             = "proxy-dns-port"
	proxyDNSMaxUpstreamConnsKey = "proxy-dns-max-upstream-conns"
	proxyDNSAddressKey          = "proxy-dns-address"
	proxyDNSUpstreamKey         = "proxy-dns-upstream"
	proxyDNSBootstrapKey        = "proxy-dns-bootstrap"
)

func runDNSProxyServer(c *cli.Context, dnsReadySignal chan struct{}, shutdownC <-chan struct{}, log *zerolog.Logger) error {
	port := c.Int(proxyDNSPortKey)
	if port <= 0 || port > 65535 {
		return errors.New("The 'proxy-dns-port' must be a valid port number in <1, 65535> range.")
	}

	maxUpstreamConnections := c.Int(proxyDNSMaxUpstreamConnsKey)
	if maxUpstreamConnections < 0 {
		return fmt.Errorf("'%s' must be 0 or higher", proxyDNSMaxUpstreamConnsKey)
	}

	listener, err := createDNSListener(c, port, maxUpstreamConnections, log)
	if err != nil {
		close(dnsReadySignal)
		return errors.Wrap(err, "Cannot create the DNS over HTTPS proxy server")
	}

	err = startDNSListener(listener, dnsReadySignal)
	if err != nil {
		return errors.Wrap(err, "Cannot start the DNS over HTTPS proxy server")
	}

	waitForShutdown(shutdownC)

	return stopDNSListener(listener, log)
}

func createDNSListener(c *cli.Context, port, maxUpstreamConnections int, log *zerolog.Logger) (tunneldns.Listener, error) {
	listener, err := tunneldns.CreateListener(
		c.String(proxyDNSAddressKey),
		uint16(port),
		c.StringSlice(proxyDNSUpstreamKey),
		c.StringSlice(proxyDNSBootstrapKey),
		maxUpstreamConnections,
		log,
	)

	return listener, err
}

func startDNSListener(listener tunneldns.Listener, dnsReadySignal chan struct{}) error {
	return listener.Start(dnsReadySignal)
}

func stopDNSListener(listener tunneldns.Listener, log *zerolog.Logger) error {
	defer func() {
		if r := recover(); r != nil {
			log.Warn().Msg("Error stopping DNS listener: %v", r)
		}
	}()

	err := listener.Stop()
	if err != nil {
		return errors.Wrap(err, "Error stopping DNS listener")
	}

	log.Info().Msg("DNS server stopped")

	return nil
}

func waitForShutdown(shutdownC <-chan struct{}) {
	<-shutdownC
}
