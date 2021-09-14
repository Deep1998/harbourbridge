// Package emulator contains wrapper for starting an in-memory cloud spanner
// emulator.
package emulator

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	instance "cloud.google.com/go/spanner/admin/instance/apiv1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"google.golang.org/grpc"

	instancepb "google.golang.org/genproto/googleapis/spanner/admin/instance/v1"
)

// Replace with relative path to binary.
const emulatorBinary = "../emulator_main"

// Options encapsulates options for the emulator wrapper.
type Options struct {
	// EmulatorAddress can be set to hostport (e.g., localhost:9010) to start
	// emulator subprocess at that address. If not set, emulator will pick it's
	// own available port.
	EmulatorAddress string

	// LogEmulatorRequests can be set to true to log requests/response from
	// emulator. Useful for debugging.
	LogEmulatorRequests bool

	// EmulatorStdout can be set to pipe output from emulator process.
	EmulatorStdout io.Writer

	// EmulatorStderr can be set to pipe errors from emulator process.
	EmulatorStderr io.Writer
}

// Emulator implements a thin layer to start and stop emulator.
type Emulator struct {
	opts Options

	// Address at which emulator process is running.
	hostport string

	// Command corresponding to in-process emulator, set if running.
	cmd *exec.Cmd

	// once is for Stop that should cleanup only once.
	once sync.Once
}

// Start starts a new cloud spanner emulator as an in-memory process.
func Start(ctx context.Context, opts Options) (emu *Emulator, err error) {
	defer func() {
		if err != nil {
			emu.Stop()
		}
	}()

	emu = &Emulator{
		opts: opts,
	}
	if err = emu.startEmulatorSubprocess(); err != nil {
		return nil, fmt.Errorf("Error bringing up emulator subprocess: %v", err)
	}

	if err = emu.waitForReady(ctx); err != nil {
		return nil, fmt.Errorf("Error waiting for emulator to start: %v", err)
	}
	fmt.Printf("Cloud spanner emulator listening at: %v", emu.hostport)
	return emu, nil
}

// Stop stops the cloud spanner emulator process. Repeated calls are a no-op.
func (emu *Emulator) Stop() {
	emu.once.Do(func() {
		if emu.cmd != nil {
			// Release resources e.g., network ports associated with the process.
			// This is required since Stop may be called even before Process.Wait()
			// returns.
			emu.cmd.Process.Release()

			// Send a kill signal to emulator process, non-blocking.
			emu.cmd.Process.Kill()
			emu.cmd = nil
			/*
				_, portStr, err := net.SplitHostPort(emu.hostport)
				if err == nil {
					port, _ := strconv.Atoi(portStr)
					portpicker.RecycleUnusedPort(port)
				}
			*/
		}
	})
}

// ClientOptions needed by go client library to talk to emulator subprocess.
func (emu *Emulator) ClientOptions() []option.ClientOption {
	return []option.ClientOption{
		option.WithEndpoint(emu.hostport),
		option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithInsecure()),
	}
}

func (emu *Emulator) startEmulatorSubprocess() error {
	emulatorPath, err := filepath.Abs(emulatorBinary)
	_, err = os.Stat(emulatorPath)
	if os.IsNotExist(err) {
		return fmt.Errorf("cannot find cloud spanner emulator binary at %v", emulatorPath)
	}

	emu.hostport = emu.opts.EmulatorAddress
	if emu.hostport == "" {
		emu.hostport = "localhost:9010"
	}

	logRequests := "--nolog_requests"
	if emu.opts.LogEmulatorRequests {
		logRequests = "--log_requests"
	}
	emu.cmd = exec.Command(emulatorPath,
		"--host_port", emu.hostport,
		logRequests)
	// Terminate the emulator server if the main process is terminated.
	emu.cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}

	if emu.opts.EmulatorStdout != nil {
		emu.cmd.Stdout = emu.opts.EmulatorStdout
	} else {
		emu.cmd.Stdout = os.Stdout
	}
	if emu.opts.EmulatorStderr != nil {
		emu.cmd.Stderr = emu.opts.EmulatorStderr
	} else {
		emu.cmd.Stderr = os.Stderr
	}

	if err := emu.cmd.Start(); err != nil {
		return fmt.Errorf("error starting emulator subprocess: %v", err)
	}
	return nil
}

func (emu *Emulator) waitForReady(ctx context.Context) error {
	timeout := 30 * time.Second
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	dialOptions := []option.ClientOption{
		option.WithGRPCDialOption(grpc.WithBlock()),
	}
	dialOptions = append(dialOptions, emu.ClientOptions()...)
	instanceAdmin, err := instance.NewInstanceAdminClient(ctx, dialOptions...)
	if err != nil {
		return fmt.Errorf("failed to create an instance admin client for emulator: %v", err)
	}

	// To test whether the server is up, wait for ListInstanceConfigs to respond
	// for a dummy project.
	configIter := instanceAdmin.ListInstanceConfigs(ctx, &instancepb.ListInstanceConfigsRequest{
		Parent: "projects/test-project",
	})
	if _, err := configIter.Next(); err != nil && err != iterator.Done {
		return fmt.Errorf("emulator failed to come up at %v within %v deadline: %v", emu.hostport, timeout.String(), err)
	}
	return nil
}
