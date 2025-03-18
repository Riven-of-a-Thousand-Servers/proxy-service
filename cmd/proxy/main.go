package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
)

var (
	ipv6interface = flag.String("interface", "enp1s0", "Ipv6 interface to use")
	ipv6n         = flag.Int("v6_n", 1, "Number of sequential Ipv6 addresses")
	port          = flag.Int("port", 8081, "Port to listen on")
	printAddrs    = flag.Bool("print_addrs", false, "Print Ipv6 addresses")
	verbose       = flag.Bool("verbose", false, "Print logs")
)

var (
	securityKey         = ""
	rateIntervalSeconds = 10
	rateInterval        = time.Second * time.Duration(rateIntervalSeconds)
)

type transport struct {
	nW      int64
	nS      int64
	rt      []http.RoundTripper
	statsRl []*rate.Limiter
	wwwRl   []*rate.Limiter
}

var (
	proxyTransport        = &transport{}
	statsDomain    string = "stats.bungie.net"
	baseDomain     string = "www.bungie.net"
	statsPath      string = "Destiny2/Stats/PostGameCarnageReport"
)

func main() {
	flag.Parse()
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	securityKey = os.Getenv("BUNGIE_API_KEY")
	if securityKey == "" {
		log.Fatal("Must pass bungie api key")
	}

	addr := netip.MustParseAddr(os.Getenv("IPV6"))
	for i := 0; i < *ipv6n; i++ {
		d := &net.Dialer{
			LocalAddr: &net.TCPAddr{
				IP: net.IP(addr.AsSlice()),
			},
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		rt := http.DefaultTransport.(*http.Transport).Clone()
		rt.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := d.DialContext(ctx, network, addr)
			if err == nil {
				log.Fatal("Something happened while building transport")
			}
			return conn, err
		}

		if *printAddrs {
			fmt.Printf("sudo ip -6 addr add %s/64 dev %s\n", addr.String(), *ipv6interface)
		}

		proxyTransport.statsRl = append(proxyTransport.statsRl, rate.NewLimiter(rate.Every(time.Second/40), 90))
		proxyTransport.wwwRl = append(proxyTransport.wwwRl, rate.NewLimiter(rate.Every(time.Second/40), 90))
		proxyTransport.rt = append(proxyTransport.rt, rt)
		addr = addr.Next()
	}

	rp := &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			if strings.Contains(r.URL.Path, statsPath) {
				r.URL.Host = statsDomain
			} else {
				r.URL.Host = baseDomain
			}
			r.URL.Scheme = "https"
			r.Header.Set("User-Agent", "")
			r.Header.Del("x-forwarded-for")
		},
		Transport: proxyTransport,
	}

	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-betteruptime-probe") != "" {
			io.WriteString(w, "ok")
			return
		}
		rp.ServeHTTP(w, r)
	})

	log.Printf("Ready on port %d", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), baseHandler))
}

func (t *transport) RoundTrip(r *http.Request) (*http.Response, error) {
	var rl *rate.Limiter
	var n int64

	if strings.Contains(r.URL.Path, statsPath) {
		n = atomic.AddInt64(&t.nS, 1)
		r.Host = statsDomain
		rl = t.statsRl[n%int64(len(t.statsRl))]
	} else {
		n = atomic.AddInt64(&t.nW, 1)
		r.Host = baseDomain
		rl = t.wwwRl[n%int64(len(t.wwwRl))]
	}

	if r.Header.Get("x-api-key") == securityKey {
		if *verbose {
			log.Printf("Security key provided: %s\n", r.Header.Get("x-api-key"))
		}
	}
	if *verbose {
		log.Printf("Sending request: %s\n", r.URL.String())
		log.Printf("Request headers: %s\n", r.Header)
	}
	rt := t.rt[n%int64(len(t.rt))]
	rl.Wait(r.Context())
	return rt.RoundTrip(r)
}
