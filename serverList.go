package main

import (
	"encoding/binary"
	"fmt"
	"html/template"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

const privateIndexFilename = "index.html"

func handleForwardedPorts(tun *tunnelCon, portCounts int) error {
	for _, listener := range listeners {
		listener.Close()
	}

	//Build port list data
	forwardedPorts = []int{}
	portsStr := ""
	listeners = []*net.UDPConn{}
	for p := range portCounts {

		//Read port
		port, err := binary.ReadUvarint(tun.frameReader)
		if err != nil {
			return fmt.Errorf("unable to read forwarded port: %v", err)
		}

		//Name length
		nameLen, err := binary.ReadUvarint(tun.frameReader)
		if err != nil {
			return fmt.Errorf("unable to read name length: %v", err)
		}

		//Read name
		var name []byte
		if nameLen > 0 {
			name = make([]byte, nameLen)
			l, err := io.ReadFull(tun.frameReader, name)
			if err != nil {
				return fmt.Errorf("Unable to read frame data: %v", err)
			}
			if l != int(nameLen) {
				return fmt.Errorf("Unable to read all frame data: %v of %v", l, nameLen)
			}
		}

		//Build list
		forwardedPorts = append(forwardedPorts, int(port))
		forwardedPortsNames = append(forwardedPortsNames, string(name))
		if p != 0 {
			portsStr = portsStr + ", "
		}
		if nameLen > 0 {
			portsStr = portsStr + string(name) + " - "
		}
		portsStr = portsStr + strconv.FormatUint(port, 10)

		//Add listener
		laddr := &net.UDPAddr{IP: nil, Port: int(port)}
		conn, err := net.ListenUDP("udp", laddr)
		if err != nil {
			return fmt.Errorf("unable to read from laddr: %v", err)
		}
		listeners = append(listeners, conn)
	}

	doLog("Forwarded ports: %v", portsStr)
	outputServerList()

	return nil
}

func outputServerList() {
	data := PageData{Servers: []ServerEntry{}}

	for i, port := range forwardedPorts {
		name := forwardedPortsNames[i]
		server := ServerEntry{Name: name, Addr: clientAddr, Port: port}
		data.Servers = append(data.Servers, server)
	}

	sourceTemplate := privateServerTemplate
	if PublicClientMode != "true" {
		sourceTemplate = publicServerTemplate
		htmlFileName = privateIndexFilename
	}

	parsedTemplate, err := template.New("page").Parse(sourceTemplate)
	if err != nil {
		doLog("Failed to parse template: %v", err)
		os.Exit(1)
	}

	f, err := os.Create(htmlFileName)
	if err != nil {
		doLog("Failed to create file: %v", err)
		os.Exit(1)
	}
	defer f.Close()

	err = parsedTemplate.Execute(f, data)
	if err != nil {
		doLog("Failed to execute template: %v", err)
		os.Exit(1)
	}

	doLog("%v written successfully.", htmlFileName)

	if PublicClientMode == "true" {
		if err := openInBrowser(htmlFileName); err != nil {
			doLog("Failed to open in browser: %v", err)
		}
	}
}

func openInBrowser(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	case "darwin":
		cmd = exec.Command("open", path)
	default: // linux, freebsd, etc.
		cmd = exec.Command("xdg-open", path)
	}

	return cmd.Start()
}
