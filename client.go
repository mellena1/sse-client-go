package sse

import (
	"errors"
	"io"
	"net/http"
	"sync"
)

var (
	// ErrStreamIsClosed is passed to the user when the stream returns an EOF
	ErrStreamIsClosed = errors.New("Stream has closed")
)

// Client is a struct to use to stream event
type Client struct {
	HTTPClient         *http.Client
	currentlyStreaming map[chan *Event]chan bool
	mutex              sync.Mutex
}

// NewClient create a new sse client given a http.Client
func NewClient(httpclient *http.Client) *Client {
	return &Client{
		HTTPClient:         httpclient,
		currentlyStreaming: make(map[chan *Event]chan bool),
		mutex:              sync.Mutex{},
	}
}

// Stream get events through a channel given a request
// If ErrStreamIsClosed is passed through the error channel, the stream is disconnected/EOF
func (c *Client) Stream(req *http.Request) (<-chan *Event, <-chan error) {
	eventch := make(chan *Event)

	c.mutex.Lock()
	c.currentlyStreaming[eventch] = make(chan bool)
	c.mutex.Unlock()

	errch := make(chan error)

	go func() {
		var resp *http.Response

		defer c.closeRespAndCurrStreamCh(resp, eventch)

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			errch <- err
			return
		}
		if resp.StatusCode != 200 {
			errch <- errors.New("non-200 status code from stream")
			return
		}

		scanner := newEventScanner(resp.Body)

		for {
			eventBytes, err := scanner.scanEvent()
			if err != nil {
				// stream no longer sending data
				if err == io.EOF {
					errch <- ErrStreamIsClosed
					return
				}

				errch <- err
				return
			}

			// readEvent only returns an error if the message should be ignored
			if event, err := readEvent(eventBytes); err == nil {
				eventch <- event
			}

			// user requested to stop the stream (non-blocking check)
			select {
			case <-c.currentlyStreaming[eventch]:
				return
			}
		}
	}()

	return eventch, errch
}

// StopStream pass in the channel used for getting the events to stop the stream
func (c *Client) StopStream(ch chan *Event) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if streamch, ok := c.currentlyStreaming[ch]; ok {
		streamch <- true
	}
}

// closeRespAndCurrStreamCh closes the response if possible and
// closes/deletes the channel used for stopping the stream
func (c *Client) closeRespAndCurrStreamCh(resp *http.Response, ch chan *Event) {
	if resp != nil {
		resp.Body.Close()
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if streamch, ok := c.currentlyStreaming[ch]; ok {
		close(streamch)
		delete(c.currentlyStreaming, ch)
	}
}
