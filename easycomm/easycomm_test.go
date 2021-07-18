package easycomm

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/w1xm/rci_interface/easycomm/simulator"
	"golang.org/x/sync/errgroup"
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
		{`IP7,1.5,.5`, Status{AzVel: 1.5, ElVel: .5}},
		{`GS262`, Status{StatusRegister: 262, Moving: true, CommandAzFlags: "POSITION", CommandElFlags: "NONE"}},
		{`GE6`, Status{ErrorRegister: 6, ErrorFlags: struct{ NoError, SensorError, HomingError, MotorError bool }{SensorError: true, HomingError: true}}},
		{`IP0,35.6`, Status{Temperature: 35.6}},
		{`IP1,2,1`, Status{AzimuthCW: true, ElevationLower: true}},
		{`IP5,10,15 IP5,11 IP5,12`, Status{RawAzDrive: 12, RawElDrive: 15}},
		{`IP7,1.5,0.5`, Status{AzVel: 1.5, ElVel: 0.5}},
		{`CR10,150,10.5`, Status{CommandAzPos: 150, CommandElPos: 10.5}},
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

func TestSimulator(t *testing.T) {
	sim, conn := simulator.New()
	ctx, cancel := context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return sim.Run(ctx)
	})
	rot := &Rotator{statusCallback: func(s Status) {}, conn: conn}
	g.Go(func() error {
		if err := rot.watch(ctx); err != io.EOF {
			t.Errorf("Rotator.watch got %v, want EOF", err)
			return err
		}
		return nil
	})
	rot.SetAzimuthPosition(150)
	time.Sleep(2 * time.Second)
	cancel()
	if err := g.Wait(); err != nil {
		t.Errorf("Wait returned %v", err)
	}
	t.Logf("final status: %+v", rot.status)
}
