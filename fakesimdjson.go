package fakesimdjson

import (
	"encoding/json"
	"math"
	"strconv"

	"github.com/minio/simdjson-go"
)

func parse(b []byte, ndjson bool) (*simdjson.ParsedJson, error) {
	p := newParser()
	p.initialize(b, ndjson)

	return p.parse()
}

// Parse a block of data and return the parsed JSON.
func Parse(b []byte) (*simdjson.ParsedJson, error) {
	return parse(b, false)
}

// ParseND will parse and return a block of newline-delimited JSON.
func ParseND(b []byte) (*simdjson.ParsedJson, error) {
	return parse(b, true)
}

func appendBool(tape []uint64, val bool) []uint64 {
	if val {
		return append(tape, uint64(simdjson.TagBoolTrue)<<simdjson.JSONTAGOFFSET)
	}
	return append(tape, uint64(simdjson.TagBoolFalse)<<simdjson.JSONTAGOFFSET)
}

func appendNumber(tape []uint64, val json.Number) ([]uint64, error) {
	i64, err := val.Int64()
	if err == nil {
		return append(tape,
			uint64(simdjson.TagInteger)<<simdjson.JSONTAGOFFSET,
			uint64(i64),
		), nil
	}
	u64, err := strconv.ParseUint(val.String(), 10, 64)
	if err == nil {
		return append(tape,
			uint64(simdjson.TagUint)<<simdjson.JSONTAGOFFSET,
			u64,
		), nil
	}
	f64, err := val.Float64()
	if err != nil {
		return nil, err
	}
	return appendFloat(tape, f64), nil
}

func appendFloat(tape []uint64, val float64) []uint64 {
	return append(tape,
		uint64(simdjson.TagFloat)<<simdjson.JSONTAGOFFSET,
		math.Float64bits(val),
	)
}

func appendString(tape []uint64, tstrings []byte, val string) ([]uint64, []byte) {
	tape = append(tape,
		((uint64(simdjson.TagString)<<simdjson.JSONTAGOFFSET)|simdjson.STRINGBUFBIT)|uint64(len(tstrings)),
		uint64(len(val)),
	)
	tstrings = append(tstrings, val...)
	return tape, tstrings
}

func appendNull(tape []uint64) []uint64 {
	return append(tape, uint64(simdjson.TagNull)<<simdjson.JSONTAGOFFSET)
}
