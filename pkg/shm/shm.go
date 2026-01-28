package shm

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"syscall"
	"time"
)

const (
	IPC_SIZE            = 0x100000 // 1MB
	IPC_CMD_RING_OFFSET = 0x00000
	IPC_RSP_RING_OFFSET = 0x08000
	IPC_DOORBELL_OFFSET = 0x10000
	IPC_RING_SIZE       = 1024
	RSP_CODE_STUB       = 0x80C9
)

type SharedMemory struct {
	file *os.File
	data []byte
}

type Packet struct {
	Cmd       uint16
	Flags     uint16
	PayloadID uint32
	Timestamp uint64
}

type Response struct {
	Status     uint16
	OrigCmd    uint16
	EntropyAvg float32
	Result     uint32
	Padding    uint32
}

type RingHeader struct {
	Magic    uint32
	Head     uint32
	Tail     uint32
	Size     uint32
	Reserved [4]uint32
}

func Connect(path string) (*SharedMemory, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open shm file: %v", err)
	}

	data, err := syscall.Mmap(int(file.Fd()), 0, IPC_SIZE, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to mmap: %v", err)
	}

	return &SharedMemory{
		file: file,
		data: data,
	}, nil
}

func (s *SharedMemory) Close() error {
	if s.data != nil {
		syscall.Munmap(s.data)
	}
	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

func (s *SharedMemory) readRingHeader(offset int) RingHeader {
	base := offset
	return RingHeader{
		Magic: binary.LittleEndian.Uint32(s.data[base : base+4]),
		Head:  binary.LittleEndian.Uint32(s.data[base+4 : base+8]),
		Tail:  binary.LittleEndian.Uint32(s.data[base+8 : base+12]),
		Size:  binary.LittleEndian.Uint32(s.data[base+12 : base+16]),
	}
}

func (s *SharedMemory) writeRingTail(offset int, tail uint32) {
	binary.LittleEndian.PutUint32(s.data[offset+8:offset+12], tail)
}

func (s *SharedMemory) readResponseAt(index uint32) *Response {
	// Ring Header is 32 bytes
	// Response struct is 16 bytes: 2+2+4+4+4
	// Status(2), OrigCmd(2), EntropyAvg(4), Result(4), Padding(4)

	packetSize := 16
	headerSize := 32
	base := IPC_RSP_RING_OFFSET + headerSize + (int(index&(IPC_RING_SIZE-1)) * packetSize)

	bitsAvg := binary.LittleEndian.Uint32(s.data[base+4 : base+8])
	entropyAvg := math.Float32frombits(bitsAvg)

	return &Response{
		Status:     binary.LittleEndian.Uint16(s.data[base : base+2]),
		OrigCmd:    binary.LittleEndian.Uint16(s.data[base+2 : base+4]),
		EntropyAvg: entropyAvg,
		Result:     binary.LittleEndian.Uint32(s.data[base+8 : base+12]),
		Padding:    binary.LittleEndian.Uint32(s.data[base+12 : base+16]),
	}
}

func (s *SharedMemory) PollResponse() (*Response, error) {
	header := s.readRingHeader(IPC_RSP_RING_OFFSET)

	// Empty check: If head == tail, no new responses
	if header.Head == header.Tail {
		// Optimization: Sleep briefly to avoid 100% CPU busy wait
		time.Sleep(1 * time.Millisecond)
		return nil, nil
	}

	// Read the response at the current tail
	resp := s.readResponseAt(header.Tail)

	// Logic for RSP_CODE_STUB (0x80C9)
	if resp.Status == RSP_CODE_STUB {
		// Trigger the VTP spec-repair flow
		// In a real scenario, we might use a channel or callback here.
		// For now, we return it so the caller can handle it.
		fmt.Printf("CODE STUB DETECTED! Result: %d Entropy: %.4f\n", resp.Result, resp.EntropyAvg)
	}

	// Increment tail to consume the response
	// We wrap naturally but just in case, we follow the u32 semantic
	s.writeRingTail(IPC_RSP_RING_OFFSET, header.Tail+1)

	return resp, nil
}

// SendCommand writes a command to the shared memory command ring
func (s *SharedMemory) SendCommand(cmd uint16, payload uint32) error {
	header := s.readRingHeader(IPC_CMD_RING_OFFSET)

	// Check if full
	if header.Head-header.Tail >= IPC_RING_SIZE {
		return fmt.Errorf("command ring full")
	}

	packetSize := 16
	headerSize := 32
	base := IPC_CMD_RING_OFFSET + headerSize + (int(header.Head&(IPC_RING_SIZE-1)) * packetSize)

	binary.LittleEndian.PutUint16(s.data[base:base+2], cmd)
	binary.LittleEndian.PutUint16(s.data[base+2:base+4], 0) // Flags
	binary.LittleEndian.PutUint32(s.data[base+4:base+8], payload)
	binary.LittleEndian.PutUint64(s.data[base+8:base+16], uint64(time.Now().UnixNano()))

	// Update Head
	// We need to write to the Head field in the command ring header
	binary.LittleEndian.PutUint32(s.data[IPC_CMD_RING_OFFSET+4:IPC_CMD_RING_OFFSET+8], header.Head+1)

	// Ring Doorbell
	// IPC_DOORBELL_OFFSET + 0 is magic, +4 version
	// +8 is cmd_doorbell
	doorbellParamsOffset := IPC_DOORBELL_OFFSET + 8
	binary.LittleEndian.PutUint32(s.data[doorbellParamsOffset:doorbellParamsOffset+4], header.Head+1)

	return nil
}
