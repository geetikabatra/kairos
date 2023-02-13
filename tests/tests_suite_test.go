package mos_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	process "github.com/mudler/go-processmanager"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/peg/matcher"
	machine "github.com/spectrocloud/peg/pkg/machine"
	"github.com/spectrocloud/peg/pkg/machine/types"
)

func TestSuite(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "kairos Test Suite")
}

func detachAndReboot() {
	vbox, ok := Machine.(*machine.VBox)
	if ok {
		vbox.DetachCD()
		vbox.Restart()
	} else {
		Reboot()
	}
}

var tempDir string
var sshPort string

var machineID string = os.Getenv("MACHINE_ID")

// https://gist.github.com/sevkin/96bdae9274465b2d09191384f86ef39d
// GetFreePort asks the kernel for a free open port that is ready to use.
func getFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

var _ = AfterSuite(func() {
	if os.Getenv("CREATE_VM") == "true" && os.Getenv("KEEP_VM") != "true" {
		fmt.Printf("Deleting VM ... %s\n", Machine.Config().StateDir)
		if Machine != nil {
			Machine.Stop()
			Machine.Clean()
		}
	}
	if !CurrentSpecReport().Failure.IsZero() {
		gatherLogs()
	}
	if os.Getenv("CREATE_VM") == "true" && os.Getenv("KEEP_VM") == "true" {
		fmt.Println("WARNING: Not cleaning", Machine.Config().StateDir)
	}
})

func user() string {
	user := os.Getenv("SSH_USER")
	if user == "" {
		user = "kairos"
	}
	return user
}

func pass() string {
	pass := os.Getenv("SSH_PASS")
	if pass == "" {
		pass = "kairos"
	}

	return pass
}

var _ = BeforeSuite(func() {

})

func gatherLogs() {
	Sudo("k3s kubectl get pods -A -o json > /run/pods.json")
	Sudo("k3s kubectl get events -A -o json > /run/events.json")
	Sudo("cat /proc/cmdline > /run/cmdline")
	Sudo("chmod 777 /run/events.json")

	Sudo("df -h > /run/disk")
	Sudo("mount > /run/mounts")
	Sudo("blkid > /run/blkid")

	GatherAllLogs(
		[]string{
			"edgevpn@kairos",
			"kairos-agent",
			"cos-setup-boot",
			"cos-setup-network",
			"kairos",
			"k3s",
		},
		[]string{
			"/var/log/edgevpn.log",
			"/var/log/kairos/agent.log",
			"/run/pods.json",
			"/run/disk",
			"/run/mounts",
			"/run/blkid",
			"/run/events.json",
			"/run/cmdline",
		})
}

func startVM() VM {
	if os.Getenv("ISO") == "" && os.Getenv("CREATE_VM") == "true" {
		fmt.Println("ISO missing")
		os.Exit(1)
	}

	vmName := uuid.New().String()

	stateDir, err := os.MkdirTemp("", "")
	Expect(err).ToNot(HaveOccurred())

	sshPort, err := getFreePort()
	Expect(err).ToNot(HaveOccurred())

	memory := os.Getenv("MEMORY")
	if memory == "" {
		memory = "2096"
	}
	cpus := os.Getenv("CPUS")
	if cpus == "" {
		cpus = "2"
	}

	opts := []types.MachineOption{
		types.QEMUEngine,
		types.WithISO(os.Getenv("ISO")),
		types.WithMemory(memory),
		types.WithCPU(cpus),
		types.WithSSHPort(strconv.Itoa(sshPort)),
		types.WithID(vmName),
		types.WithSSHUser(user()),
		types.WithSSHPass(pass()),
		types.OnFailure(func(p *process.Process) {
			out, _ := os.ReadFile(p.StdoutPath())
			err, _ := os.ReadFile(p.StderrPath())
			status, _ := p.ExitCode()

			// We are explicitly killing the qemu process. We don't treat that as an error
			// but we just print the output just in case.
			fmt.Printf("\nVM Aborted: %s %s Exit status: %s\n", out, err, status)
		}),
		types.WithStateDir(stateDir),
		types.WithDataSource(os.Getenv("DATASOURCE")),
	}

	// Set this to true to debug.
	// You can connect to it with "spicy" or other tool.
	var spicePort int
	if os.Getenv("MACHINE_SPICY") != "" {
		spicePort, err = getFreePort()
		Expect(err).ToNot(HaveOccurred())
		fmt.Printf("Spice port = %d\n", spicePort)
		opts = append(opts, types.WithDisplay(fmt.Sprintf("-spice port=%d,addr=127.0.0.1,disable-ticketing", spicePort)))
	}

	if os.Getenv("KVM") != "" {
		opts = append(opts, func(m *types.MachineConfig) error {
			m.Args = append(m.Args,
				"-enable-kvm",
			)
			return nil
		})
	}

	if os.Getenv("USE_QEMU") == "true" {
		opts = append(opts, types.QEMUEngine)
		// DISPLAY is already taken on Linux X sessions
		if os.Getenv("MACHINE_DISPLAY") == "true" {
			port, _ := getFreePort()
			display := fmt.Sprintf("-vga qxl -spice port=%d,addr=127.0.0.1,disable-ticketing=yes", port)
			opts = append(opts, types.WithDisplay(display))

		}
	} else {
		opts = append(opts, types.VBoxEngine)
	}
	m, err := machine.New(opts...)
	Expect(err).ToNot(HaveOccurred())

	vm := NewVM(m, stateDir)

	err = vm.Start(context.Background())
	Expect(err).ToNot(HaveOccurred())

	if os.Getenv("MACHINE_SPICY") != "" {
		cmd := exec.Command("spicy",
			"-h", "127.0.0.1",
			"-p", strconv.Itoa(spicePort))
		err = cmd.Start()
		Expect(err).ToNot(HaveOccurred())
	}

	return vm
}

func isFlavor(flavor string) bool {
	return strings.Contains(os.Getenv("FLAVOR"), flavor)
}
