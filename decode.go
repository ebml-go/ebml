// Copyright (c) 2012, Jorge Acereda Maciá. All rights reserved.
//
// Use of this source code is governed by a BSD-style license that can
// be found in the LICENSE file.

// Package ebml decodes EBML data.
//
// EBML is short for Extensible Binary Meta Language. EBML specifies a
// binary and octet (byte) aligned format inspired by the principle of
// XML. EBML itself is a generalized description of the technique of
// binary markup. Like XML, it is completely agnostic to any data that it
// might contain.
// For a specification, see http://ebml.sourceforge.net/specs/
package ebml

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"reflect"
	"strconv"
)

var Verbose bool = false

// ReachedPayloadError is generated when a field tagged with
// ebmlstop:"1" is reached.
type ReachedPayloadError struct {
	*Element
}

func (r ReachedPayloadError) Error() string {
	return "Reached payload"
}

// Element represents an EBML-encoded chunk of data.
type Element struct {
	*limitedReadSeeker
	Offset int64
	Id     uint
}

func (e *Element) String() string {
	return fmt.Sprintf("{ReadSeeker: %+v Offset: %+v Id: %x}", e.limitedReadSeeker, e.Offset, e.Id)
}

// Creates the root element corresponding to the data available in r.
func RootElement(rs io.ReadSeeker) (*Element, error) {
	e := &Element{newLimitedReadSeeker(rs, math.MaxInt64), 0, 0}
	return e, nil
}

func remaining(x int8) (rem int) {
	for x > 0 {
		rem++
		x += x
	}
	return
}

func parseVint(data []uint8) (val uint64) {
	for i, l := 0, len(data); i < l; i++ {
		val <<= 8
		val += uint64(data[i])
	}
	return
}

func readVintData(r io.Reader) (data []uint8, err error, rem int) {
	var v [16]uint8
	_, err = io.ReadFull(r, v[:1])
	if err == nil {
		rem = remaining(int8(v[0]))
		_, err = io.ReadFull(r, v[1:rem+1])
	}
	if err == nil {
		data = v[0 : rem+1]
	}
	return
}

func readVint(r io.Reader) (val uint64, err error, rem int) {
	var data []uint8
	data, err, rem = readVintData(r)
	if err == nil {
		val = parseVint(data)
	}
	return
}

func readSize(r io.Reader) (int64, error) {
	val, err, rem := readVint(r)
	return int64(val & ^(128 << uint(rem*8-rem))), err
}

// Next returns the next child element in an element.
func (e *Element) Next() (*Element, error) {
	if e == nil {
		return nil, io.EOF
	}
	if Verbose {
		log.Println("next", e)
	}
	off, err := e.Seek(0, 1)
	if err != nil {
		log.Panic(err)
	}
	id, err, _ := readVint(e)
	if err != nil {
		return nil, err
	}
	sz, err := readSize(e)
	if err != nil {
		return nil, err
	}
	ret := &Element{newLimitedReadSeeker(e, sz), off, uint(id)}
	if Verbose {
		log.Println("--->", ret)
	}
	return ret, err
}

func (e *Element) Size() int64 {
	return e.limitedReadSeeker.N
}

func (e *Element) readUint64() (uint64, error) {
	d, err := e.ReadData()
	var i int
	sz := len(d)
	var val uint64
	for i = 0; i < sz; i++ {
		val <<= 8
		val += uint64(d[i])
	}
	return val, err
}

func (e *Element) readUint() (uint, error) {
	val, err := e.readUint64()
	return uint(val), err
}

func (e *Element) readString() (string, error) {
	s, err := e.ReadData()
	sl := len(s)
	for sl > 0 && s[sl-1] == 0 {
		sl--
	}
	return string(s[:sl]), err
}

func (e *Element) ReadData() (d []byte, err error) {
	sz := e.Size()
	d = make([]uint8, sz, sz)
	_, err = io.ReadFull(e, d)
	return
}

func (e *Element) readFloat() (val float64, err error) {
	var uval uint64
	sz := e.Size()
	uval, err = e.readUint64()
	if sz == 8 {
		val = math.Float64frombits(uval)
	} else {
		val = float64(math.Float32frombits(uint32(uval)))
	}
	return
}

func (e *Element) skip() (err error) {
	_, err = e.Seek(e.Size(), 1)
	return
}

// Unmarshal reads EBML data from r into data. Data must be a pointer
// to a struct. Fields present in the struct but absent in the stream
// will just keep their zero value.
// Returns an error that can be an io.Error or a ReachedPayloadError.
func (e *Element) Unmarshal(val interface{}) error {
	return e.readStruct(reflect.Indirect(reflect.ValueOf(val)))
}

func getTag(f reflect.StructField, s string) uint {
	sid := f.Tag.Get(s)
	id, _ := strconv.ParseUint(sid, 16, 0)
	return uint(id)
}

func lookup(reqid uint, t reflect.Type) int {
	for i, l := 0, t.NumField(); i < l; i++ {
		f := t.Field(i)
		if getTag(f, "ebml") == reqid {
			return i - 1000000*int(getTag(f, "ebmlstop"))
		}
	}
	return -1
}

func setDefaults(v reflect.Value) {
	t := v.Type()
	for i, l := 0, t.NumField(); i < l; i++ {
		fv := v.Field(i)
		switch fv.Kind() {
		case reflect.Int, reflect.Uint,
			reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64,
			reflect.String:
			setFieldDefaults(fv, t.Field(i), v)
		case reflect.Array, reflect.Struct, reflect.Slice:
			break
		default:
			log.Panic("Unsupported type")
		}
	}
}

func setFieldDefaults(v reflect.Value, sf reflect.StructField, s reflect.Value) {
	if v.CanInterface() && reflect.DeepEqual(
		v.Interface(), reflect.Zero(v.Type()).Interface()) {
		tag := sf.Tag.Get("ebmldef")
		if tag != "" {
			switch v.Kind() {
			case reflect.Int, reflect.Int64:
				u, _ := strconv.ParseInt(tag, 10, 0)
				v.SetInt(int64(u))
			case reflect.Uint, reflect.Uint64:
				u, _ := strconv.ParseUint(tag, 10, 0)
				v.SetUint(u)
			case reflect.Float32, reflect.Float64:
				f, _ := strconv.ParseFloat(tag, 64)
				v.SetFloat(f)
			case reflect.String:
				v.SetString(tag)
			default:
				log.Panic("Unsupported default value")
			}
		}
		ltag := sf.Tag.Get("ebmldeflink")
		if ltag != "" {
			v.Set(s.FieldByName(ltag))
		}
	}
}

func (e *Element) readStruct(v reflect.Value) (err error) {
	t := v.Type()
	for err == nil {
		var ne *Element
		ne, err = e.Next()
		if err == io.EOF {
			err = nil
			break
		}
		i := lookup(ne.Id, t)
		if i >= 0 {
			err = ne.readField(v.Field(i))
		} else if i == -1 {
			err = ne.skip()
		} else {
			var curr int64
			curr, err = e.Seek(ne.Offset, 0)
			if err != nil || curr != ne.Offset {
				log.Panic(err, curr)
			}
			err = ReachedPayloadError{e}
		}
	}
	setDefaults(v)
	return
}

func (e *Element) readField(v reflect.Value) (err error) {
	switch v.Kind() {
	case reflect.Struct:
		err = e.readStruct(v)
	case reflect.Slice:
		err = e.readSlice(v)
	case reflect.Array:
		for i, l := 0, v.Len(); i < l && err == nil; i++ {
			err = e.readStruct(v.Index(i))
		}
	case reflect.String:
		var s string
		s, err = e.readString()
		v.SetString(s)
	case reflect.Int, reflect.Int64:
		var u uint64
		u, err = e.readUint64()
		v.SetInt(int64(u))
	case reflect.Uint, reflect.Uint64:
		var u uint64
		u, err = e.readUint64()
		v.SetUint(u)
	case reflect.Float32, reflect.Float64:
		var f float64
		f, err = e.readFloat()
		v.SetFloat(f)
	default:
		err = errors.New("Unknown type: " + v.String())
	}
	return
}

func (e *Element) readSlice(v reflect.Value) (err error) {
	switch v.Type().Elem().Kind() {
	case reflect.Uint8:
		var sl []uint8
		sl, err = e.ReadData()
		if err == nil {
			if !v.CanSet() {
				log.Panic("can't set ", v, e)
			}
			v.Set(reflect.ValueOf(sl))
		}
	case reflect.Struct:
		vl := v.Len()
		ne := reflect.New(v.Type().Elem())
		nsl := reflect.Append(v, reflect.Indirect(ne))
		v.Set(nsl)
		err = e.readStruct(v.Index(vl))
	default:
		err = errors.New("Unknown slice type: " + v.String())
	}
	return
}
