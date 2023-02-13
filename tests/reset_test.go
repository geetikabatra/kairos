package mos_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/spectrocloud/peg/matcher"
)

var _ = Describe("kairos reset test", Label("reset-test"), func() {
	var vm VM
	BeforeEach(func() {
		if os.Getenv("CLOUD_INIT") == "" || !filepath.IsAbs(os.Getenv("CLOUD_INIT")) {
			Fail("CLOUD_INIT must be set and must be pointing to a file as an absolute path")
		}

		vm = startVM()
		vm.EventuallyConnects(1200)
	})

	Context("live cd", func() {
		It("has default service active", func() {
			if isFlavor("alpine") {
				out, _ := vm.Sudo("rc-status")
				Expect(out).Should(ContainSubstring("kairos"))
				Expect(out).Should(ContainSubstring("kairos-agent"))
				fmt.Println(out)
			} else {
				// Eventually(func() string {
				// 	out, _ := Machine.Command("sudo systemctl status kairososososos-agent")
				// 	return out
				// }, 30*time.Second, 10*time.Second).Should(ContainSubstring("no network token"))

				out, _ := vm.Sudo("systemctl status kairos")
				Expect(out).Should(ContainSubstring("loaded (/etc/systemd/system/kairos.service; enabled;"))
				fmt.Println(out)
			}

			out, _ := vm.Sudo("ls -liah /oem")
			fmt.Println(out)
			//	Expect(out).To(ContainSubstring("userdata.yaml"))
			out, _ = vm.Sudo("cat /oem/userdata")
			fmt.Println(out)
			out, _ = vm.Sudo("ps aux")
			fmt.Println(out)

			out, _ = vm.Sudo("lsblk")
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
	})

	Context("reboots and passes functional tests", func() {
		It("has grubenv file", func() {
			Eventually(func() string {
				out, _ := vm.Sudo("cat /oem/grubenv")
				return out
			}, 40*time.Minute, 1*time.Second).Should(
				Or(
					ContainSubstring("foobarzz"),
				))
		})

		It("resets", func() {
			_, err := vm.Sudo("touch /usr/local/test")
			Expect(err).ToNot(HaveOccurred())

			_, err = vm.Sudo("touch /oem/test")
			Expect(err).ToNot(HaveOccurred())

			HasFile("/oem/test")
			HasFile("/usr/local/test")

			_, err = vm.Sudo("grub2-editenv /oem/grubenv set next_entry=statereset")
			Expect(err).ToNot(HaveOccurred())

			Reboot()

			Eventually(func() string {
				out, _ := vm.Sudo("if [ -f /usr/local/test ]; then echo ok; else echo wrong; fi")
				return out
			}, 40*time.Minute, 1*time.Second).Should(
				Or(
					ContainSubstring("wrong"),
				))
			Eventually(func() string {
				out, _ := vm.Sudo("if [ -f /oem/test ]; then echo ok; else echo wrong; fi")
				return out
			}, 40*time.Minute, 1*time.Second).Should(
				Or(
					ContainSubstring("ok"),
				))
		})
	})
})
