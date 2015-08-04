package gokeepasslib

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"encoding/xml"
	"errors"
	"io"
	"io/ioutil"
	"reflect"
)

// Decoder stores a reader which is expected to be in kdbx format
type Decoder struct {
	r io.Reader
}

func (d *Decoder) Decode(db *Database) error {
	s, err := ReadSignature(d.r)
	if err != nil {
		return err
	}
	db.Signature = s

	db.Headers = new(FileHeaders)
	err = db.Headers.ReadFrom(d.r)
	if err != nil {
		return err
	}

	if err := d.readData(db); err != nil {
		return err
	}

	return nil
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

func (d *Decoder) readData(db *Database) error {
	masterKey, err := db.Credentials.buildMasterKey(db)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return err
	}

	in, err := ioutil.ReadAll(d.r)
	if err != nil {
		return err
	}

	mode := cipher.NewCBCDecrypter(block, db.Headers.EncryptionIV)
	decrypted := make([]byte, len(in))
	mode.CryptBlocks(decrypted, in)

	startBytes := db.Headers.StreamStartBytes
	if !reflect.DeepEqual(decrypted[0:len(startBytes)], startBytes) {
		return errors.New("Database integrity check failed")
	}
	decrypted = decrypted[len(startBytes):]

	var xmlDecoder *xml.Decoder
	if db.Headers.CompressionFlags == GzipCompressionFlag { //Unzip if the header compression flag is 1 for gzip
		zippedBody, err := DecodeBlocks(decrypted)
		if err != nil {
			return err
		}

		b := bytes.NewBuffer(zippedBody)
		r, err := gzip.NewReader(b)
		if err != nil {
			return err
		}
		defer r.Close()
		xmlDecoder = xml.NewDecoder(r)
	} else { //Otherwise assume it not compressed
		xmlDecoder = xml.NewDecoder(bytes.NewReader(decrypted))
	}

	db.Content = &DBContent{}
	err = xmlDecoder.Decode(db.Content)
	return err
}
