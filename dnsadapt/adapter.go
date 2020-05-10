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
	"github.com/miekg/dns"
)

// ---------------------------------------------------------------------------------------
//  types
// ---------------------------------------------------------------------------------------

type DnsAdapter func(handler dns.Handler) dns.Handler

// ---------------------------------------------------------------------------------------
//  public functions
// ---------------------------------------------------------------------------------------

// Chain chains a http.Handler with the given adapters.
func Chain(h dns.Handler, adapters ...DnsAdapter) dns.Handler {
	for _, adapter := range adapters {
		h = adapter(h)
	}
	return h
}

// ChainFunc chains a http.HandlerFunc with the given adapater.
func ChainFunc(h dns.HandlerFunc, adapters ...DnsAdapter) dns.Handler {
	return Chain(dns.HandlerFunc(h), adapters...)
}
