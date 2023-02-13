package mos_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/peg/matcher"
)

var _ = Describe("kairos autoinstall test", Label("autoinstall-test"), func() {
	var vm VM

	BeforeEach(func() {
		if os.Getenv("CLOUD_INIT") == "" || !filepath.IsAbs(os.Getenv("CLOUD_INIT")) {
			Fail("CLOUD_INIT must be set and must be pointing to a file as an absolute path")
		}

		vm = startVM()
		vm.EventuallyConnects(1200)

	})

	AfterEach(func() {
		if CurrentGinkgoTestDescription().Failed {
			gatherLogs()
		}
	})

	Context("live cd", func() {
		It("has default service active", func() {
			if strings.Contains(os.Getenv("FLAVOR"), "alpine") {
				out, _ := vm.Sudo("rc-status")
				Expect(out).Should(ContainSubstring("kairos"))
				Expect(out).Should(ContainSubstring("kairos-agent"))
				fmt.Println(out)
			} else {
				// Eventually(func() string {
				// 	out, _ := machine.Command("sudo systemctl status kairososososos-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := vm.Sudo("systemctl status kairos")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos.service; enabled;"))
				fmt.Println(out)
			}

			// Debug output
			out, _ := vm.Sudo("ls -liah /oem")
			fmt.Println(out)
			//	Expect(out).To(ContainSubstring("userdata.yaml"))
			out, _ = vm.Sudo("cat /oem/userdata")
			fmt.Println(out)
			out, _ = vm.Sudo("sudo ps aux")
			fmt.Println(out)

			out, _ = vm.Sudo("sudo lsblk")
			fmt.Println(out)
		})
	})

	Context("auto installs", func() {
		It("to disk with custom config", func() {
			Eventually(func() string {
				out, _ := vm.Sudo("ps aux")
				return out
			}, 30*time.Minute, 1*time.Second).Should(
				Or(
					ContainSubstring("elemental install"),
				))
		})
		It("reboots to active", func() {
			Eventually(func() string {
				out, _ := vm.Sudo("kairos-agent state boot")
				return out
			}, 40*time.Minute, 10*time.Second).Should(
				Or(
					ContainSubstring("active_boot"),
				))
		})
	})

	Context("reboots and passes functional tests", func() {
		It("has grubenv file", func() {
			out, err := vm.Sudo("cat /oem/grubenv")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("foobarzz"))

		})

		It("has custom cmdline", func() {
			out, err := vm.Sudo("cat /proc/cmdline")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("foobarzz"))
		})

		It("uses the dracut immutable module", func() {
			out, err := vm.Sudo("cat /proc/cmdline")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("cos-img/filename="))
		})

		It("installs Auto assessment", func() {
			// Auto assessment was installed
			out, _ := vm.Sudo("cat /run/initramfs/cos-state/grubcustom")
			Expect(out).To(ContainSubstring("bootfile_loc"))

			out, _ = vm.Sudo("cat /run/initramfs/cos-state/grub_boot_assessment")
			Expect(out).To(ContainSubstring("boot_assessment_blk"))

			cmdline, _ := vm.Sudo("cat /proc/cmdline")
			Expect(cmdline).To(ContainSubstring("rd.emergency=reboot rd.shell=0 panic=5"))
		})

		It("has writeable tmp", func() {
			_, err := vm.Sudo("echo 'foo' > /tmp/bar")
			Expect(err).ToNot(HaveOccurred())

			out, err := Machine.Command("sudo cat /tmp/bar")
			Expect(err).ToNot(HaveOccurred())

			Expect(out).To(ContainSubstring("foo"))
		})

		It("has bpf mount", func() {
			out, err := vm.Sudo("mount")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("bpf"))
		})

		It("has correct permissions", func() {
			out, err := vm.Sudo(`stat -c "%a" /oem`)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("770"))

			out, err = vm.Sudo(`stat -c "%a" /usr/local/cloud-config`)
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("770"))
		})

		It("has grubmenu", func() {
			out, err := vm.Sudo("cat /run/initramfs/cos-state/grubmenu")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("state reset"))
		})

		It("has additional mount specified, with no dir in rootfs", func() {
			out, err := vm.Sudo("mount")
			Expect(err).ToNot(HaveOccurred())
			Expect(out).To(ContainSubstring("/var/lib/longhorn"))
		})

		It("has rootfs shared mount", func() {
			out, err := vm.Sudo(`cat /proc/1/mountinfo | grep ' / / '`)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).To(ContainSubstring("shared"))
		})

		It("doesn't has grub data into the cloud config", func() {
			out, err := vm.Sudo(`cat /oem/90_custom.yaml`)
			Expect(err).ToNot(HaveOccurred(), out)
			Expect(out).ToNot(ContainSubstring("vga_text"))
			Expect(out).ToNot(ContainSubstring("videotest"))
		})

		It("has corresponding state", func() {
			out, err := vm.Sudo("kairos-agent state")
			Expect(err).ToNot(HaveOccurred())
			fmt.Println(out)
			Expect(out).To(ContainSubstring("boot: active_boot"))

			stateAssertVM(vm, "oem.mounted", "true")
			stateAssertVM(vm, "oem.found", "true")
			stateAssertVM(vm, "persistent.mounted", "true")
			stateAssertVM(vm, "state.mounted", "true")
			stateAssertVM(vm, "oem.type", "ext4")
			stateAssertVM(vm, "persistent.type", "ext4")
			stateAssertVM(vm, "state.type", "ext4")
			stateAssertVM(vm, "oem.mount_point", "/oem")
			stateAssertVM(vm, "persistent.mount_point", "/usr/local")
			stateAssertVM(vm, "persistent.name", "/dev/vda")
			stateAssertVM(vm, "state.mount_point", "/run/initramfs/cos-state")
			stateAssertVM(vm, "oem.read_only", "false")
			stateAssertVM(vm, "persistent.read_only", "false")
			stateAssertVM(vm, "state.read_only", "true")
		})
	})
})
