package main

// swarm-dns-sd
// Copyright (C) 2020 Maximilian Pachl

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

// ---------------------------------------------------------------------------------------
//  imports
// ---------------------------------------------------------------------------------------

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/faryon93/util"
	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"

	"github.com/faryon93/kallax/dnsadapt"
	"github.com/faryon93/kallax/metric"
	"github.com/faryon93/kallax/store"
)

// ---------------------------------------------------------------------------------------
//  constants
// ---------------------------------------------------------------------------------------

const (
	BaseDomain = "kallax.local"
)

// ---------------------------------------------------------------------------------------
//  global variables
// ---------------------------------------------------------------------------------------

var (
	Colors     bool
	Debug	   bool
	DockerHost string
	DnsListen  string
	PromListen string

	Store store.Store

	reEndpointName = regexp.MustCompile("([A-Za-z0-9_-]+)\\.task-(\\d+)-([A-Za-z0-9_-]+)\\.([A-Za-z0-9_-]+)\\.([A-Za-z0-9_-]+)\\.([A-Za-z0-9_-]+)\\.kallax\\.local")
)

// ---------------------------------------------------------------------------------------
//  private functions
// ---------------------------------------------------------------------------------------

func MakeSrvRRFromEndpoint(q *dns.Question, ep *store.Endpoint) (dns.RR, error) {
	// TTL IN SRV priority weight port target
	return dns.NewRR(fmt.Sprintf("%s 15 IN SRV 10 0 %d %s.%s.",
		q.Name, ep.Port, ep.Name, BaseDomain))
}

func handleDnsQuery(w dns.ResponseWriter, r *dns.Msg) {
	// only handle DNS Queries
	if r.Opcode != dns.OpcodeQuery {
		return
	}

	// parse and handle the query
	m := new(dns.Msg)
	m.SetReply(r)
	m.Compress = false

	// answer all questions if possible
	for _, q := range m.Question {
		logrus.Debugf("Query for \"%s\" (%d)", q.Name, q.Qtype)

		switch q.Qtype {
		case dns.TypeA:
			// TODO: sanitize input
			pp := reEndpointName.FindStringSubmatch(q.Name)
			if len(pp) < 1 {
				return
			}

			taskId := pp[3]
			networkId := pp[6]
			addr, err := Store.GetTaskIpAddresses(taskId, networkId)
			if err != nil {
				logrus.Errorln("failed to get task ip addresses:", err.Error())
				return
			}

			record := fmt.Sprintf("%s 15 IN A %s", q.Name, addr)
			rr, err := dns.NewRR(record)
			if err != nil {
				logrus.Error("failed to construct DNS A-RR:", err.Error())
				continue
			}
			m.Answer = append(m.Answer, rr)
			logrus.Debugf("%s", addr)

		case dns.TypeSRV:
			// TODO: sanitize input
			groupName := strings.TrimRight(q.Name, BaseDomain+".")
			eps, err := Store.GetGroupEndpoints(groupName)
			if err != nil {
				logrus.Errorln("failed to query group endpoints:", err.Error())
				return
			}

			for _, ep := range eps {
				rr, err := MakeSrvRRFromEndpoint(&q, ep)
				if err != nil {
					logrus.Error("failed to construct DNS SRV-RR:", err.Error())
					continue
				}
				m.Answer = append(m.Answer, rr)

				logrus.Debugf("%s:%d", ep.Name, ep.Port)
			}
		}
	}

	err := w.WriteMsg(m)
	if err != nil {
		logrus.Error("failed to write dns response:", err.Error())
	}
}

// ---------------------------------------------------------------------------------------
//  application entry
// ---------------------------------------------------------------------------------------

func main() {
	flag.BoolVar(&Colors, "color", false, "force color logging")
	flag.BoolVar(&Debug, "debug", false, "turn on debug log")
	flag.StringVar(&DockerHost, "docker", "unix:///var/run/docker.sock", "docker host")
	flag.StringVar(&DnsListen, "dns-listen", ":5353", "dns udp listen")
	flag.StringVar(&PromListen, "prom-listen", ":9800", "prometheus http listen")
	flag.Parse()

	// setup logger
	formater := logrus.TextFormatter{ForceColors: Colors}
	logrus.SetFormatter(&formater)
	logrus.SetOutput(os.Stdout)
	if Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.Infoln("starting", GetAppVersion())

	var err error
	Store, err = store.NewDocker(client.WithHost(DockerHost), client.WithAPIVersionNegotiation())
	if err != nil {
		logrus.Errorln("failed to create docker swarm store:", err.Error())
		os.Exit(-1)
	}
	logrus.Infoln("connected to docker on", DockerHost)

	// start prometheus metrics endpoint
	if PromListen != "" {
		go func() {
			logrus.Infoln("listening \"prom-metrics\" on", PromListen)
			http.Handle("/metrics", promhttp.Handler())
			err := http.ListenAndServe(PromListen, nil)
			if err != nil {
				logrus.Errorln("metrics endpoint failed:", err.Error())
			}
		}()
	}

	// start DNS server
	server := &dns.Server{Addr: DnsListen, Net: "udp"}
	go func() {
		logrus.Infoln("listening \"dns\" on", DnsListen)
		chain := dnsadapt.ChainFunc(handleDnsQuery, dnsadapt.PromHistogram(metric.ProcessingTime))
		dns.Handle(BaseDomain+".", chain)
		err = server.ListenAndServe()
		if err != nil {
			logrus.Fatalf("failed to start DNS server: %s\n ", err.Error())
			os.Exit(-1)
		}
	}()
	defer server.Shutdown()

	util.WaitSignal(os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	logrus.Println("received SIGINT / SIGTERM going to shutdown")
}
