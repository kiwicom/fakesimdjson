package fakesimdjson

import (
	"testing"

	"github.com/minio/simdjson-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		json string
		err  string
	}{
		{
			name: "empty object",
			json: `{}`,
		},
		{
			name: "single null",
			json: `{"value": null}`,
		},
		{
			name: "single true",
			json: `{"value": true}`,
		},
		{
			name: "single false",
			json: `{"value": false}`,
		},
		{
			name: "single int",
			json: `{"value": -5}`,
		},
		{
			name: "single zero",
			json: `{"value": 0}`,
		},
		{
			name: "single float",
			json: `{"value": 1.3}`,
		},
		{
			name: "single uint",
			json: `{"value": 18446744073709551615}`, // max uint64
		},
		{
			name: "single string",
			json: `{"value": "hello world"}`,
		},
		{
			name: "single empty object",
			json: `{"value": {}}`,
		},
		{
			name: "single empty array",
			json: `{"value": []}`,
		},
		{
			name: "malformed object",
			json: `{"value"}`,
			err:  "invalid character '}' after object key",
		},
		{
			name: "malformed object 2",
			json: `{"value":}`,
			err:  "invalid character '}' looking for beginning of value",
		},
		{
			name: "malformed object 3",
			json: `{"value":`,
			err:  "unexpected EOF",
		},
		{
			name: "malformed object 4",
			json: `{`,
			err:  "unexpected EOF",
		},
		{
			name: "malformed object 5",
			json: `{a`,
			err:  "invalid character 'a'",
		},
		{
			name: "malformed object 6",
			json: `{5: 10}`,
			err:  "invalid character '5'",
		},
		{
			name: "malformed array",
			json: `{"val":[`,
			err:  "unexpected EOF",
		},
		{
			name: "nested",
			json: `{"a": [{"msg": "hello"}, {"msg": "world"}]}`,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			ourPJ, err := Parse([]byte(test.json))
			if test.err != "" {
				require.EqualError(t, err, test.err)
				return
			}
			require.NoError(t, err, "parse our tape")
			require.NotNil(t, ourPJ)

			origPJ, err := simdjson.Parse([]byte(test.json), nil)
			require.NoError(t, err)
			require.NotNil(t, origPJ)

			origJSON, err := tape2json(origPJ)
			require.NoError(t, err)

			ourJSON, err := tape2json(ourPJ)
			require.NoError(t, err)

			assert.Equal(t, string(origJSON), string(ourJSON))
		})
	}
}

func tape2json(pj *simdjson.ParsedJson) ([]byte, error) {
	rootIt := pj.Iter()
	rootIt.Advance()
	return rootIt.MarshalJSON()
}
