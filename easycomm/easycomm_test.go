package easycomm

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type NoopCloser struct {
	io.Reader
	write bytes.Buffer
}

func (nc *NoopCloser) Write(p []byte) (n int, err error) {
	return nc.write.Write(p)
}

func (nc NoopCloser) Close() error {
	return nil
}

func TestParsing(t *testing.T) {
	for _, test := range []struct {
		input  string
		status Status
	}{
		{"AZ170.00", Status{AzPos: 170}},
		{"EL45", Status{ElPos: 45}},
		{`\?VEL1.5,.5`, Status{AzVel: 1.5, ElVel: .5}},
		{`GS262`, Status{StatusRegister: 262, Moving: true, CommandAzFlags: "POSITION", CommandElFlags: "NONE"}},
	} {
		t.Run(test.input, func(t *testing.T) {
			ctx := context.Background()
			conn := &NoopCloser{
				Reader: strings.NewReader(test.input),
			}
			var status Status
			r := &Rotator{
				conn: conn,
				statusCallback: func(s Status) {
					status = s
				},
			}
			if err := r.watch(ctx); err != io.EOF {
				t.Errorf("watch failed: got %v, want EOF", err)
			}
			if diff := cmp.Diff(status, test.status); diff != "" {
				t.Errorf("unexpected status: got(-)/want(+):\n%s", diff)
			}
		})
	}
}
