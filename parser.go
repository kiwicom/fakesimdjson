package fakesimdjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/minio/simdjson-go"
)

type parser struct {
	pj *simdjson.ParsedJson
	d  *json.Decoder

	ndjson bool
}

func newParser() *parser {
	p := &parser{
		pj: &simdjson.ParsedJson{},
	}

	return p
}

func (p *parser) initialize(msg []byte, ndjson bool) {
	p.pj.Message = bytes.TrimSpace(msg)
	p.pj.Tape = make([]uint64, 0, 128)
	p.pj.Strings = &simdjson.TStrings{B: make([]byte, 0, 128)}

	p.ndjson = ndjson

	p.d = json.NewDecoder(bytes.NewReader(p.pj.Message))
	p.d.UseNumber()
}

func (p *parser) parse() (*simdjson.ParsedJson, error) {
	tok, err := p.d.Token()
	for {
		if err != nil {
			if errors.Is(err, io.EOF) {
				if len(p.pj.Tape) == 0 {
					return nil, io.ErrUnexpectedEOF
				}
				return p.pj, nil
			}
			return nil, err
		}

		startIndex := len(p.pj.Tape)
		p.pj.Tape = append(p.pj.Tape, 0)
		switch {
		case errors.Is(err, io.EOF):
			return nil, io.ErrUnexpectedEOF
		case err != nil:
			return nil, err
		}

		delim, ok := tok.(json.Delim)
		if !ok || (delim != '{' && delim != '[') {
			return nil, fmt.Errorf("expecting object or array, got %T %v", tok, tok)
		}

		if delim == '{' {
			err = p.parseObject()
		} else {
			err = p.parseArray()
		}
		if err != nil {
			return nil, err
		}

		p.pj.Tape = append(p.pj.Tape, uint64(simdjson.TagRoot)<<simdjson.JSONTAGOFFSET)
		size := uint64(len(p.pj.Tape)) - uint64(startIndex)
		p.pj.Tape[startIndex] = (uint64(simdjson.TagRoot) << simdjson.JSONTAGOFFSET) | size

		tok, err = p.d.Token()

		if !p.ndjson {
			if !errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("expecting EOF, got %T %v", tok, tok)
			}
			return p.pj, nil
		}
	}
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
