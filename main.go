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
	"fmt"
	"log"
	"os"
	"regexp"
	"syscall"

	"github.com/docker/docker/client"
	"github.com/faryon93/kallax/store"
	"github.com/faryon93/util"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
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
	Store store.Store

	reEndpointName = regexp.MustCompile("([A-Za-z0-9_-]+)\\.task-(\\d+)-([A-Za-z0-9_-]+)\\.([A-Za-z0-9_-]+)\\.([A-Za-z0-9_-]+)\\.kallax\\.local")
)

// ---------------------------------------------------------------------------------------
//  private functions
// ---------------------------------------------------------------------------------------

func MakeSrvRRFromEndpoint(q *dns.Question, ep *store.Endpoint) (dns.RR, error) {
	return dns.NewRR(fmt.Sprintf("%s 15 IN SRV 10 60 %d %s.%s",
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
		log.Printf("Query for %s\n", q.Name)

		switch q.Qtype {
		case dns.TypeA:
			pp := reEndpointName.FindStringSubmatch(q.Name)
			if len(pp) < 1 {
				return
			}

			taskId := pp[3]
			addrs, err := Store.GetTaskIpAddresses(taskId)
			if err != nil {
				logrus.Errorln("failed to get task ip addresses:", err.Error())
				return
			}

			// todo: richtig ip addresse auswaehlen
			for _, addr := range addrs {
				record := fmt.Sprintf("%s A %s", q.Name, addr)
				rr, err := dns.NewRR(record)
				if err != nil {
					logrus.Error("failed to construct DNS A-RR:", err.Error())
					continue
				}
				m.Answer = append(m.Answer, rr)
				log.Printf("\t-> %s", addr)
			}

		case dns.TypeSRV:
			eps, err := Store.GetGroupEndpoints("test")
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

				log.Println("\t->", ep.Name)
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
	var err error
	Store, err = store.NewDocker(client.WithHost("tcp://10.0.28.186:2376"))
	if err != nil {
		logrus.Errorln("failed to create docker swarm store:", err.Error())
		return
	}

	// start he DNS server
	dns.HandleFunc(".", handleDnsQuery)
	server := &dns.Server{Addr: ":5454", Net: "udp"}
	err = server.ListenAndServe()
	defer server.Shutdown()
	if err != nil {
		log.Fatalf("Failed to start server: %s\n ", err.Error())
	}

	util.WaitSignal(os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Println("received SIGINT / SIGTERM going to shutdown")
}
