package espat

import (
	"bytes"
	"reflect"
	"testing"
)

type writeCmdTest struct {
	cmd  string
	args []any
	out  string
	err  error
}

var writeCmdTests = []writeCmdTest{
	{"", nil, `AT`, nil},
	{"+RESTORE", nil, `AT+RESTORE`, nil},
	{"+CWMODE=", []any{1, 0}, `AT+CWMODE=1,0`, nil},
	{"+CWJAP=", []any{"SSID", "password", nil, 1, 2, nil, 1}, `AT+CWJAP="SSID","password",,1,2,,1`, nil},
	{"+CIPSERVER=1,", []any{1234}, `AT+CIPSERVER=1,1234`, nil},
	{"+SLEEP=", []any{'a'}, "", ErrArgType},
	{"+CIPTCPOPT=", []any{-1, 0, 999}, `AT+CIPTCPOPT=-1,0,999`, nil},
}

func TestWriteCmd(t *testing.T) {
	var buf [128]byte
	w := bytes.NewBuffer(nil)
	for _, test := range writeCmdTests {
		w.Reset()
		err := writeCmd(w, &buf, test.cmd, test.args)
		if test.err != nil {
			if !reflect.DeepEqual(err, test.err) {
				t.Errorf(
					"%s %+v -> %#v: errors don't match: %v",
					test.cmd, test.args, test.out, test.err,
				)
			}
		} else if err != nil {
			t.Errorf(
				"%s %+v -> %#v: unexpected error: %v",
				test.cmd, test.args, test.out, err,
			)
		}
		tout := test.out
		if tout != "" {
			tout += "\r\n"
		}
		if out := w.String(); out != tout {
			t.Errorf(
				"%s %+v -> %#v != %#v",
				test.cmd, test.args, tout, out,
			)
		}
	}
}
