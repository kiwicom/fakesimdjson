package fakesimdjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"

	"github.com/minio/simdjson-go"
)

// Parse a block of data and return the parsed JSON.
func Parse(b []byte) (*simdjson.ParsedJson, error) {
	p := parser{
		d: json.NewDecoder(bytes.NewReader(b)),
		pj: &simdjson.ParsedJson{
			Message: b,
			Tape:    make([]uint64, 0, 128),
			Strings: &simdjson.TStrings{B: make([]byte, 0, 128)},
		},
	}
	p.d.UseNumber()

	p.pj.Tape = append(p.pj.Tape, 0) // root tag will be replaced at the end
	tok, err := p.d.Token()
	switch {
	case errors.Is(err, io.EOF):
		return nil, io.ErrUnexpectedEOF
	case err != nil:
		return nil, err
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("expecting object, got %T %v", tok, tok)
	}
	err = p.parseObject()
	if err != nil {
		return nil, err
	}
	tok, err = p.d.Token()
	if !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("expecting EOF, got %T %v", tok, tok)
	}
	p.pj.Tape = append(p.pj.Tape, uint64(simdjson.TagRoot)<<simdjson.JSONTAGOFFSET)
	size := uint64(len(p.pj.Tape))
	p.pj.Tape[0] = (uint64(simdjson.TagRoot) << simdjson.JSONTAGOFFSET) | size
	return p.pj, nil
}

type parser struct {
	d  *json.Decoder
	pj *simdjson.ParsedJson
}

// parseValue parses the value in tok and appends it to the tape.
func (p *parser) parseValue(tok json.Token) error {
	switch val := tok.(type) {
	case json.Delim:
		switch val {
		case '{':
			return p.parseObject()
		case '[':
			return p.parseArray()
		default:
			panic("end delimiter should never be reached")
		}
	case bool:
		p.pj.Tape = appendBool(p.pj.Tape, val)
	case json.Number:
		var err error
		p.pj.Tape, err = appendNumber(p.pj.Tape, val)
		if err != nil {
			return err
		}
	case string:
		p.pj.Tape, p.pj.Strings.B = appendString(p.pj.Tape, p.pj.Strings.B, val)
	case nil:
		p.pj.Tape = appendNull(p.pj.Tape)
	default:
		panic(fmt.Sprintf("unsupported %T value token: %v", tok, tok))
	}
	return nil
}

// parseObject parses object contents after { until and including } and appends the object to the tape.
func (p *parser) parseObject() error {
	startOffset := len(p.pj.Tape)
	p.pj.Tape = append(p.pj.Tape, 0) // will be replaced when we exit the object
	for {
		tok, err := p.d.Token()
		switch {
		case errors.Is(err, io.EOF):
			return io.ErrUnexpectedEOF
		case err != nil:
			return err
		}
		if delim, ok := tok.(json.Delim); ok && delim == '}' {
			break
		}
		key, ok := tok.(string)
		if !ok {
			return fmt.Errorf("expecting string key, got %T %v", tok, tok)
		}
		p.pj.Tape, p.pj.Strings.B = appendString(p.pj.Tape, p.pj.Strings.B, key)
		tok, err = p.d.Token()
		switch {
		case errors.Is(err, io.EOF):
			return io.ErrUnexpectedEOF
		case err != nil:
			return err
		}
		err = p.parseValue(tok)
		if err != nil {
			return err
		}
	}
	p.pj.Tape = append(p.pj.Tape, uint64(simdjson.TagObjectEnd)<<simdjson.JSONTAGOFFSET|uint64(startOffset))
	p.pj.Tape[startOffset] = (uint64(simdjson.TagObjectStart) << simdjson.JSONTAGOFFSET) | uint64(len(p.pj.Tape))
	return nil
}

// parseArray parses array contents after [ until and including ] and appends the array to the tape.
func (p *parser) parseArray() error {
	startOffset := len(p.pj.Tape)
	p.pj.Tape = append(p.pj.Tape, 0) // will be replaced when we exit the object
	for {
		tok, err := p.d.Token()
		switch {
		case errors.Is(err, io.EOF):
			return io.ErrUnexpectedEOF
		case err != nil:
			return err
		}
		if delim, ok := tok.(json.Delim); ok && delim == ']' {
			break
		}
		err = p.parseValue(tok)
		if err != nil {
			return err
		}
	}
	p.pj.Tape = append(p.pj.Tape, uint64(simdjson.TagArrayEnd)<<simdjson.JSONTAGOFFSET|uint64(startOffset))
	p.pj.Tape[startOffset] = (uint64(simdjson.TagArrayStart) << simdjson.JSONTAGOFFSET) | uint64(len(p.pj.Tape))
	return nil
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
