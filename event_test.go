package sse

import "testing"

func Test_readEvent(t *testing.T) {
	tests := []struct {
		testname  string
		input     string
		expected  *Event
		shouldErr bool
	}{
		{
			"data and event",
			"event: update\ndata: this is some test data hello, world\n",
			&Event{
				LastEventID: "",
				Type:        "update",
				Data:        []byte("this is some test data hello, world"),
			},
			false,
		},
		{
			"data,event,type and keep-alives",
			": keep-alive\n: keep-alive\nevent: add\n: keep-alive\ndata:testing 1,2,3\nid: 65\n: keep-alive\n",
			&Event{
				LastEventID: "65",
				Type:        "add",
				Data:        []byte("testing 1,2,3"),
			},
			false,
		},
		{
			"empty data",
			": keep-alive\ndata\n",
			&Event{
				LastEventID: "",
				Type:        "",
				Data:        []byte(""),
			},
			false,
		},
		{
			"no data",
			"",
			&Event{},
			true,
		},
	}

	for _, test := range tests {
		actual, err := readEvent([]byte(test.input))
		if !test.shouldErr {
			ok(t, err)
			equals(t, test.expected, actual)
		} else {
			assert(t, err != nil, "should've errored but didn't")
		}
	}
}
