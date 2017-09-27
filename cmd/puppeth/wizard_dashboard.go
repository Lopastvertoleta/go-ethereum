// Copyright 2017 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"

	"github.com/Lopastvertoleta/go-ethereum/log"
)

// deployDashboard queries the user for various input on deploying a web-service
// dashboard, after which is pushes the container.
func (w *wizard) deployDashboard() {
	// Select the server to interact with
	server := w.selectServer()
	if server == "" {
		return
	}
	client := w.servers[server]

	// Retrieve any active dashboard configurations from the server
	infos, err := checkDashboard(client, w.network)
	if err != nil {
		infos = &dashboardInfos{
			port: 80,
			host: client.server,
		}
	}
	// Figure out which port to listen on
	fmt.Println()
	fmt.Printf("Which port should the dashboard listen on? (default = %d)\n", infos.port)
	infos.port = w.readDefaultInt(infos.port)

	// Figure which virtual-host to deploy the dashboard on
	infos.host, err = w.ensureVirtualHost(client, infos.port, infos.host)
	if err != nil {
		log.Error("Failed to decide on dashboard host", "err", err)
		return
	}
	// Port and proxy settings retrieved, figure out which services are available
	available := make(map[string][]string)
	for server, services := range w.services {
		for _, service := range services {
			available[service] = append(available[service], server)
		}
	}
	listing := make(map[string]string)
	for _, service := range []string{"ethstats", "explorer", "wallet", "faucet"} {
		// Gather all the locally hosted pages of this type
		var pages []string
		for _, server := range available[service] {
			client := w.servers[server]
			if client == nil {
				continue
			}
			// If there's a service running on the machine, retrieve it's port number
			var port int
			switch service {
			case "ethstats":
				if infos, err := checkEthstats(client, w.network); err == nil {
					port = infos.port
				}
			case "faucet":
				if infos, err := checkFaucet(client, w.network); err == nil {
					port = infos.port
				}
			}
			if page, err := resolve(client, w.network, service, port); err == nil && page != "" {
				pages = append(pages, page)
			}
		}
		// Promt the user to chose one, enter manually or simply not list this service
		defLabel, defChoice := "don't list", len(pages)+2
		if len(pages) > 0 {
			defLabel, defChoice = pages[0], 1
		}
		fmt.Println()
		fmt.Printf("Which %s service to list? (default = %s)\n", service, defLabel)
		for i, page := range pages {
			fmt.Printf(" %d. %s\n", i+1, page)
		}
		fmt.Printf(" %d. List external %s service\n", len(pages)+1, service)
		fmt.Printf(" %d. Don't list any %s service\n", len(pages)+2, service)

		choice := w.readDefaultInt(defChoice)
		if choice < 0 || choice > len(pages)+2 {
			log.Error("Invalid listing choice, aborting")
			return
		}
		switch {
		case choice <= len(pages):
			listing[service] = pages[choice-1]
		case choice == len(pages)+1:
			fmt.Println()
			fmt.Printf("Which address is the external %s service at?\n", service)
			listing[service] = w.readString()
		default:
			// No service hosting for this
		}
	}
	// If we have ethstats running, ask whether to make the secret public or not
	var ethstats bool
	if w.conf.ethstats != "" {
		fmt.Println()
		fmt.Println("Include ethstats secret on dashboard (y/n)? (default = yes)")
		ethstats = w.readDefaultString("y") == "y"
	}
	// Try to deploy the dashboard container on the host
	if out, err := deployDashboard(client, w.network, infos.port, infos.host, listing, &w.conf, ethstats); err != nil {
		log.Error("Failed to deploy dashboard container", "err", err)
		if len(out) > 0 {
			fmt.Printf("%s\n", out)
		}
		return
	}
	// All ok, run a network scan to pick any changes up
	w.networkStats(false)
}
