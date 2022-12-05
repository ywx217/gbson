package gbson

import (
	"bytes"
	"encoding/binary"
	"math"
	"time"
	"unsafe"

	"github.com/pkg/errors"
)

// Parse bson values quickly and efficiently.
// Inspired by tidwall/gjson
//
// BSON format specification: https://bsonspec.org/spec.html

var (
	ErrInvalidLength = errors.New("invalid length")
	ErrNotObject     = errors.New("not an object")
)

type Type uint8

const (
	BSONTypeDouble              Type = 0x01
	BSONTypeString              Type = 0x02
	BSONTypeObject              Type = 0x03
	BSONTypeArray               Type = 0x04
	BSONTypeBinary              Type = 0x05
	BSONTypeUndefined           Type = 0x06
	BSONTypeObjectID            Type = 0x07
	BSONTypeBoolean             Type = 0x08
	BSONTypeDateTime            Type = 0x09
	BSONTypeNull                Type = 0x0A
	BSONTypeRegex               Type = 0x0B
	BSONTypeDBPointer           Type = 0x0C
	BSONTypeJavaScript          Type = 0x0D
	BSONTypeSymbol              Type = 0x0E
	BSONTypeJavaScriptWithScope Type = 0x0F
	BSONTypeInt32               Type = 0x10
	BSONTypeTimestamp           Type = 0x11
	BSONTypeInt64               Type = 0x12
	BSONTypeDecimal128          Type = 0x13
	BSONTypeMinKey              Type = 0xFF
	BSONTypeMaxKey              Type = 0x7F
)

type Result struct {
	Type Type
	Raw  []byte // value part
}

// Get gets the first value by the given path.
func Get(pb []byte, path ...string) Result {
	state := resultFromBytes(pb)
	if len(path) == 0 {
		return state
	}
	return state.Get(path...)
}

// Get gets the first value by the given path.
func (r Result) Get(path ...string) (result Result) {
	result.Type = BSONTypeUndefined
	// use callback to avoid heap memory allocation
	_ = r.GetIter(func(r Result) bool {
		result = r
		return false
	}, path...)
	return
}

// GetIter gets all the values until the resultSink returns false.
// Get calls this method internally.
func (r Result) GetIter(resultSink func(Result) bool, path ...string) (err error) {
	// use recursion calls to iterate through the data in depth first order.
	var skip bool
	var depth int
	var walkFunc func([]byte, Result) bool
	walkFunc = func(key []byte, it Result) bool {
		if skip {
			return false
		}
		if !bytesEqualToString(key, path[depth]) {
			// not the desired field
			return true
		}
		if depth == len(path)-1 {
			if !resultSink(it) {
				skip = true
				return false
			}
			return true
		}
		// recursion call
		depth++
		if _, innerErr := it.iterFields(walkFunc); innerErr != nil {
			err = innerErr
			skip = true
			return false
		}
		depth--
		return true
	}
	if _, innerErr := r.iterFields(walkFunc); innerErr != nil {
		return innerErr
	}
	return nil
}

func resultFromBytes(bs []byte) Result {
	return Result{Type: BSONTypeObject, Raw: bs}
}

func consumeElement(bs []byte) (tp Type, name []byte, value []byte, totalLen int) {
	if len(bs) == 0 { // empty binary
		return BSONTypeUndefined, nil, nil, -1
	}
	tp = Type(bs[0])
	if !(tp >= 0x01 && tp <= 0x13) && tp != 0xFF && tp != 0x7F { // invalid type
		return BSONTypeUndefined, nil, nil, -1
	}
	name, nameLen := consumeCString(bs[1:])
	if nameLen == 0 { // read e_name failed
		return BSONTypeUndefined, nil, nil, -1
	}
	bs = bs[nameLen+1:]
	var valueLen int
	switch tp {
	case BSONTypeUndefined, BSONTypeNull, BSONTypeMinKey, BSONTypeMaxKey:
		valueLen = 0
	case BSONTypeBoolean:
		valueLen = 1
	case BSONTypeInt32:
		valueLen = 4
	case BSONTypeDouble, BSONTypeDateTime, BSONTypeTimestamp, BSONTypeInt64:
		valueLen = 8
	case BSONTypeObjectID:
		valueLen = 12
	case BSONTypeDecimal128:
		valueLen = 16
	case BSONTypeString, BSONTypeJavaScript, BSONTypeSymbol:
		valueLen = int(consumeInt32(bs) + 4) // string length + 4 bytes for string length
	case BSONTypeObject, BSONTypeArray:
		valueLen = int(consumeInt32(bs))
	case BSONTypeBinary:
		valueLen = int(consumeInt32(bs) + 4 + 1) // 4 for length, 1 for subtype
	case BSONTypeDBPointer:
		valueLen = int(consumeInt32(bs) + 4 + 12) // 4 for length, 12 for object id
	case BSONTypeRegex:
		_, firstLen := consumeCString(bs)
		_, secondLen := consumeCString(bs[firstLen:])
		valueLen = firstLen + secondLen
	case BSONTypeJavaScriptWithScope:
		valueLen = int(consumeInt32(bs))
	default:
		return BSONTypeUndefined, nil, nil, -1
	}
	if len(bs) < valueLen {
		return BSONTypeUndefined, nil, nil, -1
	}
	return tp, name, bs[:valueLen], 1 + nameLen + valueLen
}

func consumeCString(bs []byte) (value []byte, totalLen int) {
	idx := bytes.IndexByte(bs, 0)
	if idx == -1 {
		return nil, 0
	}
	return bs[:idx], idx + 1
}

func consumeInt32(bs []byte) (value int32) {
	if len(bs) < 4 {
		return 0
	}
	return int32(binary.LittleEndian.Uint32(bs))
}

// iterFields read through the binary data stored in r.Raw field-by-field.
func (r Result) iterFields(resultSink func(key []byte, r Result) bool) (int, error) {
	var field Result
	var consumedLength int
	var bs []byte
	if r.Type == BSONTypeObject || r.Type == BSONTypeArray {
		totalLength := consumeInt32(r.Raw)
		if len(r.Raw) < int(totalLength) {
			return 0, ErrInvalidLength
		}
		bs = r.Raw[4 : totalLength-1]
	} else {
		return 0, ErrNotObject
	}
	// fields are not organized in order, so we need to iterate through all fields
	for len(bs) > 0 {
		tp, name, value, totalLen := consumeElement(bs)
		if totalLen < 0 {
			// error occurred when totalLen is negative
			return consumedLength, ErrInvalidLength
		}
		bs = bs[totalLen:]
		consumedLength += totalLen
		field.Type = tp
		field.Raw = value
		if !resultSink(name, field) {
			return consumedLength, nil
		}
	}
	return consumedLength, nil
}

func bytesEqualToString(left []byte, right string) bool {
	return *(*string)(unsafe.Pointer(&left)) == right
}

func (r Result) Exist() bool {
	return r.Type != BSONTypeUndefined
}

func (r Result) String() string {
	if r.Type == BSONTypeString {
		return string(r.Raw[4 : len(r.Raw)-1])
	}
	return ""
}

func (r Result) Bool() bool {
	if r.Type == BSONTypeBoolean && r.Raw[0] == 0x01 {
		return true
	}
	return false
}

func (r Result) Float64() float64 {
	if r.Type == BSONTypeDouble {
		return math.Float64frombits(binary.LittleEndian.Uint64(r.Raw))
	}
	if r.Type == BSONTypeInt32 {
		return float64(r.Int32())
	}
	if r.Type == BSONTypeInt64 {
		return float64(r.Int64())
	}
	return 0
}

func (r Result) Int32() int32 {
	if r.Type == BSONTypeInt32 {
		return int32(binary.LittleEndian.Uint32(r.Raw))
	}
	if r.Type == BSONTypeInt64 {
		return int32(r.Int64())
	}
	if r.Type == BSONTypeDouble {
		return int32(r.Float64())
	}
	return 0
}

func (r Result) Int64() int64 {
	if r.Type == BSONTypeInt64 {
		return int64(binary.LittleEndian.Uint64(r.Raw))
	}
	if r.Type == BSONTypeInt32 {
		return int64(r.Int32())
	}
	if r.Type == BSONTypeDouble {
		return int64(r.Float64())
	}
	return 0
}

func (r Result) Time() time.Time {
	if r.Type == BSONTypeDateTime {
		return time.Unix(0, r.Int64()*int64(time.Millisecond))
	}
	if r.Type == BSONTypeTimestamp {
		return time.Unix(int64(binary.LittleEndian.Uint32(r.Raw[4:8])), 0)
	}
	return time.Time{}
}

func (r Result) IterArray(consumer func(Result) bool) {
	if r.Type != BSONTypeArray {
		return
	}
	_, _ = r.iterFields(func(_ []byte, r Result) bool {
		return consumer(r)
	})
}

func (r Result) IterDocument(consumer func(key string, r Result) bool) {
	if r.Type != BSONTypeObject {
		return
	}
	_, _ = r.iterFields(func(key []byte, r Result) bool {
		return consumer(string(key), r)
	})
}

func (r Result) Array() []Result {
	a := make([]Result, 0)
	r.IterArray(func(r Result) bool {
		a = append(a, r)
		return true
	})
	return a
}

func (r Result) Map() map[string]Result {
	m := make(map[string]Result)
	r.IterDocument(func(key string, r Result) bool {
		m[key] = r
		return true
	})
	return m
}

func (r Result) SizedArray(size int) []Result {
	if size == 0 {
		size = r.Length()
	}
	a := make([]Result, 0, size)
	r.IterArray(func(r Result) bool {
		a = append(a, r)
		return true
	})
	return a
}

func (r Result) SizedMap(size int) map[string]Result {
	if size == 0 {
		size = r.Length()
	}
	m := make(map[string]Result, size)
	r.IterDocument(func(key string, r Result) bool {
		m[key] = r
		return true
	})
	return m
}

func (r Result) Length() int {
	if r.Type == BSONTypeObject || r.Type == BSONTypeArray {
		var count int
		_, _ = r.iterFields(func(_ []byte, _ Result) bool {
			count++
			return true
		})
		return count
	}
	return 0
}
