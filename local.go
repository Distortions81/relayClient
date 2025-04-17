package main

import (
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"time"
)

func cleanEphemeralMaps() {
	go func() {
		ticker := time.NewTicker(ephemeralTicker)

		for range ticker.C {
			ephemeralLock.Lock()
			for key, item := range ephemeralPortMap {
				if time.Since(item.lastUsed) > ephemeralLife {
					if debugLog {
						doLog("Deleted idle ephemeral port: %v: -> %v", item.id, item.source)
					}
					delete(ephemeralPortMap, key)
				}
			}
			for key, item := range ephemeralIDMap {
				if time.Since(item.lastUsed) > ephemeralLife {
					doLog("Deleted idle ephemeral id: %v: -> %v", item.id, item.source)
					delete(ephemeralIDMap, key)
					ephemeralIDRecycle = append(ephemeralIDRecycle, key)
					ephemeralIDRecycleLen++
				}
			}
			ephemeralLock.Unlock()
		}
	}()
}

func createEphemeralID() int {
	if ephemeralIDRecycleLen > 0 {
		recycledID := ephemeralIDRecycle[0]
		ephemeralIDRecycle = ephemeralIDRecycle[1:]
		ephemeralIDRecycleLen--
		if debugLog {
			doLog("Recycling ephemeral ID %v", recycledID)
		}
		return recycledID
	} else {
		newID := ephemeralTop
		ephemeralTop++
		return newID
	}
}

func handleListeners(tun *tunnelCon) {
	for _, port := range listeners {
		go func(p *net.UDPConn) {
			if debugLog {
				defer doLog("handleListeners: exit")
			}
			for p != nil {
				// Read payload
				buf := make([]byte, bufferSizeUDP)
				n, addr, err := p.ReadFromUDP(buf)
				if err != nil {
					if debugLog {
						doLog("Error reading: %v", err)
					}
					return
				}
				if n == 0 {
					if debugLog {
						doLog("Ignoring empty packet: %v", addr)
					}
					continue
				}

				// Check ephemeral map
				ephemeralLock.Lock()
				var newSession *ephemeralData
				session := ephemeralPortMap[addr.String()]

				// New session, create
				if session == nil {
					eID := createEphemeralID()

					newSession = &ephemeralData{
						id: eID, source: addr.String(),
						destPort: getPortStr(p.LocalAddr().String()),
						lastUsed: time.Now(), listener: port}

					if publicMode {
						if tun.con == nil {
							doLog("Reconnecting on-demand.")
							go tunnelConnect()
						}
					}

					ephemeralPortMap[addr.String()] = newSession
					ephemeralIDMap[eID] = newSession

					session = newSession
					doLog("NEW SESSION ID: %v: %v -> %v", newSession.id, newSession.source, newSession.destPort)
				} else {
					if verboseDebug {
						doLog("Session ID: %v: %vb: %v -> %v", session.id, n, session.source, session.destPort)
					}
					session.lastUsed = time.Now()
				}
				ephemeralLock.Unlock()

				/* New client, tell server clientID destination */
				var header []byte
				if newSession != nil {
					header = binary.AppendUvarint(header, 0)
					header = binary.AppendUvarint(header, uint64(newSession.destPort))
				}

				//Write standard header
				header = binary.AppendUvarint(header, uint64(session.id))
				header = binary.AppendUvarint(header, uint64(n))
				tun.Write(append(header, buf[:n]...))
			}
		}(port)
	}
}

// Get port from address string
func getPortStr(input string) int {
	parts := strings.Split(input, ":")
	numparts := len(parts)
	portStr := parts[numparts-1]
	port, _ := strconv.ParseUint(portStr, 10, 64)
	return int(port)
}
