package id

import (
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"time"
)

type Id [16]byte

var (
	machineID uint64
	counter   uint32
)

// SetMachineId may only be called by one thread before any id generation
// is done. It must be set if multiple machines are generating ids in order
// to avoid collisions. Only the least significant 48 bits are used.
func SetMachineId(ID uint64) {
	machineID = ID
}

// SetMachineIdHost is a convenience wrapper to hide bit twiddling of
// calling SetMachineId, it has the same constraints as SetMachineId
// with an addition that net.IP must be a ipv4 address.
func SetMachineIdHost(addr net.IP, port uint16) {
	var machineID uint64 // 48 bits
	machineID |= uint64(addr[0]) << 40
	machineID |= uint64(addr[1]) << 32
	machineID |= uint64(addr[2]) << 24
	machineID |= uint64(addr[3]) << 16
	machineID |= uint64(port)

	SetMachineId(machineID)
}

// New will generate a new Id for use. New is safe to be called from
// concurrent threads. SetMachineId should be called once before any calls to
// New are made. 2^32 calls to New per millisecond will be unique, provided
// machine id is seeded correctly across machines.
//
// binary format: [ [ 48 bits time ] [ 48 bits machineID ] [ 32 bits counter ] ]
//
// Ids are sortable within (not between, thanks to clocks) each machine, with
// a modified base32 encoding exposed for convenience in API usage.
func New() Id {
	// NewWithTime will be inlined
	return NewWithTime(time.Now())
}

// NewWithTime returns an id that uses the milliseconds from the given time.
// New is identical to NewWithTime(time.Now())
func NewWithTime(t time.Time) Id {
	// NOTE compiler optimizes out division by constant for us
	ms := uint64(t.Unix())*1000 + uint64(t.Nanosecond()/int(time.Millisecond))
	count := atomic.AddUint32(&counter, 1)
	return newID(ms, machineID, count)
}

func newID(ms, machineID uint64, count uint32) Id {
	var id Id

	id[0] = byte(ms >> 40)
	id[1] = byte(ms >> 32)
	id[2] = byte(ms >> 24)
	id[3] = byte(ms >> 16)
	id[4] = byte(ms >> 8)
	id[5] = byte(ms)

	id[6] = byte(machineID >> 40)
	id[7] = byte(machineID >> 32)
	id[8] = byte(machineID >> 24)
	id[9] = byte(machineID >> 16)
	id[10] = byte(machineID >> 8)
	id[11] = byte(machineID)

	id[12] = byte(count >> 24)
	id[13] = byte(count >> 16)
	id[14] = byte(count >> 8)
	id[15] = byte(count)

	return id
}

// following encodings are slightly modified from https://github.com/oklog/ulid

// String returns a lexicographically sortable string encoded Id
// (26 characters, non-standard base 32) e.g. 01AN4Z07BY79KA1307SR9X4MV3
// Format: ttttttttttmmmmmmmmmmeeeeee where t is time, m is machine id
// and c is a counter
func (id Id) String() string {
	var b [EncodedSize]byte
	_ = id.MarshalTextTo(b[:])
	return string(b[:])
}

// MarshalBinary implements the encoding.BinaryMarshaler interface by
// returning the Id as a byte slice.
func (id Id) MarshalBinary() ([]byte, error) {
	var b [EncodedSize]byte
	return b[:], id.MarshalBinaryTo(b[:])
}

// MarshalBinaryTo writes the binary encoding of the Id to the given buffer.
// ErrBufferSize is returned when the len(dst) != 16.
func (id Id) MarshalBinaryTo(dst []byte) error {
	if len(dst) != len(id) {
		return errors.New("provided buffer not large enough to marshal id")
	}

	copy(dst, id[:])
	return nil
}

// UnmarshalBinary implements the encoding.BinaryUnmarshaler interface by
// copying the passed data and converting it to an Id. ErrDataSize is
// returned if the data length is different from Id length.
func (id *Id) UnmarshalBinary(data []byte) error {
	if len(data) != len(*id) {
		return errors.New("can't unmarshal id from unexpected byte slice size")
	}

	copy((*id)[:], data)
	return nil
}

// Encoding is the base 32 encoding alphabet used in Id strings.
const Encoding = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// MarshalText implements the encoding.TextMarshaler interface by
// returning the string encoded Id.
func (id Id) MarshalText() ([]byte, error) {
	var b [EncodedSize]byte
	return b[:], id.MarshalTextTo(b[:])
}

// MarshalTextTo writes the Id as a string to the given buffer.
// an error is returned when the len(dst) != 26.
func (id Id) MarshalTextTo(dst []byte) error {
	// Optimized unrolled loop ahead.
	// From https://github.com/RobThree/NUlid

	if len(dst) != EncodedSize {
		return errors.New("not enough bytes to marshal id to")
	}

	// 10 byte timestamp
	dst[0] = Encoding[(id[0]&224)>>5]
	dst[1] = Encoding[id[0]&31]
	dst[2] = Encoding[(id[1]&248)>>3]
	dst[3] = Encoding[((id[1]&7)<<2)|((id[2]&192)>>6)]
	dst[4] = Encoding[(id[2]&62)>>1]
	dst[5] = Encoding[((id[2]&1)<<4)|((id[3]&240)>>4)]
	dst[6] = Encoding[((id[3]&15)<<1)|((id[4]&128)>>7)]
	dst[7] = Encoding[(id[4]&124)>>2]
	dst[8] = Encoding[((id[4]&3)<<3)|((id[5]&224)>>5)]
	dst[9] = Encoding[id[5]&31]

	// 16 bytes of entropy
	dst[10] = Encoding[(id[6]&248)>>3]
	dst[11] = Encoding[((id[6]&7)<<2)|((id[7]&192)>>6)]
	dst[12] = Encoding[(id[7]&62)>>1]
	dst[13] = Encoding[((id[7]&1)<<4)|((id[8]&240)>>4)]
	dst[14] = Encoding[((id[8]&15)<<1)|((id[9]&128)>>7)]
	dst[15] = Encoding[(id[9]&124)>>2]
	dst[16] = Encoding[((id[9]&3)<<3)|((id[10]&224)>>5)]
	dst[17] = Encoding[id[10]&31]
	dst[18] = Encoding[(id[11]&248)>>3]
	dst[19] = Encoding[((id[11]&7)<<2)|((id[12]&192)>>6)]
	dst[20] = Encoding[(id[12]&62)>>1]
	dst[21] = Encoding[((id[12]&1)<<4)|((id[13]&240)>>4)]
	dst[22] = Encoding[((id[13]&15)<<1)|((id[14]&128)>>7)]
	dst[23] = Encoding[(id[14]&124)>>2]
	dst[24] = Encoding[((id[14]&3)<<3)|((id[15]&224)>>5)]
	dst[25] = Encoding[id[15]&31]

	return nil
}

// Byte to index table for O(1) lookups when unmarshaling.
// We use 0xFF as sentinel value for invalid indexes.
var dec = [...]byte{
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x00, 0x01,
	0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E,
	0x0F, 0x10, 0x11, 0xFF, 0x12, 0x13, 0xFF, 0x14, 0x15, 0xFF,
	0x16, 0x17, 0x18, 0x19, 0x1A, 0xFF, 0x1B, 0x1C, 0x1D, 0x1E,
	0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0x0A, 0x0B, 0x0C,
	0x0D, 0x0E, 0x0F, 0x10, 0x11, 0xFF, 0x12, 0x13, 0xFF, 0x14,
	0x15, 0xFF, 0x16, 0x17, 0x18, 0x19, 0x1A, 0xFF, 0x1B, 0x1C,
	0x1D, 0x1E, 0x1F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
}

// EncodedSize is the length of a text encoded Id.
const EncodedSize = 26

// ValidateText returns true if the data is a valid
// encoding.
func ValidateText(v []byte) bool {
	return len(v) == EncodedSize &&
		dec[v[0]] != 0xFF && dec[v[1]] != 0xFF &&
		dec[v[2]] != 0xFF && dec[v[3]] != 0xFF &&
		dec[v[4]] != 0xFF && dec[v[5]] != 0xFF &&
		dec[v[6]] != 0xFF && dec[v[7]] != 0xFF &&
		dec[v[8]] != 0xFF && dec[v[9]] != 0xFF &&
		dec[v[10]] != 0xFF && dec[v[11]] != 0xFF &&
		dec[v[12]] != 0xFF && dec[v[13]] != 0xFF &&
		dec[v[14]] != 0xFF && dec[v[15]] != 0xFF &&
		dec[v[16]] != 0xFF && dec[v[17]] != 0xFF &&
		dec[v[18]] != 0xFF && dec[v[19]] != 0xFF &&
		dec[v[20]] != 0xFF && dec[v[21]] != 0xFF &&
		dec[v[22]] != 0xFF && dec[v[23]] != 0xFF &&
		dec[v[24]] != 0xFF && dec[v[25]] != 0xFF
}

// UnmarshalText implements the encoding.TextUnmarshaler interface by
// parsing the data as string encoded Id.
//
// an error is returned if the len(v) is different from an encoded
// Id's length. Invalid encodings produce undefined Ids.
func (id *Id) UnmarshalText(v []byte) error {
	// Optimized unrolled loop ahead.
	// From https://github.com/RobThree/NUlid
	if len(v) != EncodedSize {
		return errors.New("id to unmarshal is of unexpected size")
	}

	// 6 bytes timestamp (48 bits)
	(*id)[0] = ((dec[v[0]] << 5) | dec[v[1]])
	(*id)[1] = ((dec[v[2]] << 3) | (dec[v[3]] >> 2))
	(*id)[2] = ((dec[v[3]] << 6) | (dec[v[4]] << 1) | (dec[v[5]] >> 4))
	(*id)[3] = ((dec[v[5]] << 4) | (dec[v[6]] >> 1))
	(*id)[4] = ((dec[v[6]] << 7) | (dec[v[7]] << 2) | (dec[v[8]] >> 3))
	(*id)[5] = ((dec[v[8]] << 5) | dec[v[9]])

	// 10 bytes of entropy (80 bits)
	(*id)[6] = ((dec[v[10]] << 3) | (dec[v[11]] >> 2))
	(*id)[7] = ((dec[v[11]] << 6) | (dec[v[12]] << 1) | (dec[v[13]] >> 4))
	(*id)[8] = ((dec[v[13]] << 4) | (dec[v[14]] >> 1))
	(*id)[9] = ((dec[v[14]] << 7) | (dec[v[15]] << 2) | (dec[v[16]] >> 3))
	(*id)[10] = ((dec[v[16]] << 5) | dec[v[17]])
	(*id)[11] = ((dec[v[18]] << 3) | dec[v[19]]>>2)
	(*id)[12] = ((dec[v[19]] << 6) | (dec[v[20]] << 1) | (dec[v[21]] >> 4))
	(*id)[13] = ((dec[v[21]] << 4) | (dec[v[22]] >> 1))
	(*id)[14] = ((dec[v[22]] << 7) | (dec[v[23]] << 2) | (dec[v[24]] >> 3))
	(*id)[15] = ((dec[v[24]] << 5) | dec[v[25]])

	return nil
}

// reverse encoding useful for sorting, descending
var rEncoding = reverseString(Encoding)

func reverseString(input string) string {
	// rsc: http://groups.google.com/group/golang-nuts/browse_thread/thread/a0fb81698275eede

	// Get Unicode code points.
	n := 0
	rune := make([]rune, len(input))
	for _, r := range input {
		rune[n] = r
		n++
	}
	rune = rune[0:n]
	// Reverse
	for i := 0; i < n/2; i++ {
		rune[i], rune[n-1-i] = rune[n-1-i], rune[i]
	}

	// Convert back to UTF-8.
	return string(rune)
}

// EncodeDescending returns a lexicographically sortable descending encoding
// of a given id, e.g. 000 -> ZZZ, which allows reversing the sort order when stored
// contiguously since ids are lexicographically sortable. The returned string will
// be of len(src), and assumes src is from the base32 crockford alphabet, otherwise
// using 0xFF.
func EncodeDescending(src string) string {
	var buf [EncodedSize]byte
	copy(buf[:], src)
	for i, s := range buf[:len(src)] {
		// XXX(reed): optimize as dec is
		j := strings.Index(Encoding, string(s))
		buf[i] = rEncoding[j]
	}
	return string(buf[:len(src)])
}
