package compress

import (
	"bytes"
	"encoding/base64"

	"github.com/cespare/go-smaz"
)

// smaz.Zip Compresses with SMAZ from a utf8 []byte
//
// @encode: true = encode to base64
func Zip(b []byte, encode ...bool) []byte {
	b = append([]byte{0, 218, 7}, b...)
	b = smaz.Compress(b)
	if len(encode) != 0 && encode[0] == true {
		return []byte(base64.StdEncoding.EncodeToString(b))
	}
	return b
}

// smaz.UnZip Decompresses with SMAZ from a utf8 []byte
//
// this method will try to decode from base64, or will skip that step if it fails
// (this means you do not have to know if you encoded your string to base64 on compression)
func UnZip(b []byte) ([]byte, error) {
	if dec, err := base64.StdEncoding.DecodeString(string(b)); err == nil {
		if dec, err = smaz.Decompress(dec); err == nil {
			if bytes.HasPrefix(dec, []byte{0, 218, 7}) {
				return dec[3:], nil
			}
		}
	}

	if dec, err := smaz.Decompress(b); err == nil {
		if bytes.HasPrefix(dec, []byte{0, 218, 7}) {
			return dec[3:], nil
		}
	}

	return []byte{}, smaz.ErrDecompression
}
