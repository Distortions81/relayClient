package main

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"time"
)

func (tun *tunnelCon) Write(buf []byte) {
	if tun == nil {
		return
	}

	tun.packetLock.Lock()
	if batchingMicroseconds == 0 {
		tun.packets = buf
		tun.packetsLength = len(buf)
		err := writeBatch(tun)
		if err != nil {
			log.Printf("Write: %v", err)
		}
	} else {
		tun.packets = append(tun.packets, buf...)
		tun.packetsLength += len(buf)
	}
	tun.packetLock.Unlock()
}

func (tun *tunnelCon) batchWriter() {
	if batchingMicroseconds == 0 {
		return
	}
	ticker := time.NewTicker(time.Microsecond * time.Duration(batchingMicroseconds))

	for range ticker.C {
		if tun == nil || tun.con == nil {
			return
		}
		tun.packetLock.Lock()
		err := writeBatch(tun)
		tun.packetLock.Unlock()
		if err != nil {
			return
		}
	}
}

func writeBatch(tun *tunnelCon) error {

	if tun.packetsLength == 0 {
		return nil
	}

	var dataToWrite []byte
	if compressionLevel > 0 {
		dataToWrite = compressFrame(tun)
	} else {
		// Write raw data, no compression
		dataToWrite = tun.packets
	}

	var header []byte
	header = binary.AppendUvarint(header, uint64(len(dataToWrite)))
	l, err := tun.con.Write(append(header, dataToWrite...))
	if err != nil {
		return fmt.Errorf("batchWrite: Write error: %v", err)
	}

	if l < len(dataToWrite) {
		return fmt.Errorf("batchWrite: Partial write: wrote %d of %d bytes", l, len(dataToWrite))
	}

	tun.packets = nil
	tun.packetsLength = 0
	return nil
}

func compressFrame(tun *tunnelCon) []byte {
	// Compress the data
	var buf bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&buf, compressionLevels[compressionLevel])
	if _, err := gz.Write(tun.packets); err != nil {
		log.Printf("batchWrite: gzip write error: %v", err)
		_ = gz.Close()
		return nil
	}
	if err := gz.Close(); err != nil {
		log.Printf("batchWrite: gzip close error: %v", err)
		return nil
	}
	return buf.Bytes()
}

func decompressFrame(data []byte) ([]byte, error) {
	buf := bytes.NewReader(data)
	gz, err := gzip.NewReader(buf)
	if err != nil {
		return nil, fmt.Errorf("gzip reader error: %v", err)
	}
	defer gz.Close()

	var out bytes.Buffer
	if _, err := io.Copy(&out, gz); err != nil {
		return nil, fmt.Errorf("gzip decompress copy error: %v", err)
	}
	return out.Bytes(), nil
}
