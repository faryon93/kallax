package dnsadapt

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
	"time"

	"github.com/miekg/dns"
	"github.com/prometheus/client_golang/prometheus"
)

// ---------------------------------------------------------------------------------------
//  public functions
// ---------------------------------------------------------------------------------------

func PromHistogram(hist prometheus.Histogram) DnsAdapter {
	return func(h dns.Handler) dns.Handler {
		return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			startTime := time.Now()
			h.ServeDNS(w, r)
			hist.Observe(time.Since(startTime).Seconds())
		})
	}
}
