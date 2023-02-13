package mos_test

import (
	. "github.com/spectrocloud/peg/matcher"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("kairos netboot test", Label("netboot-test"), func() {
	var vm VM
	BeforeEach(func() {
		vm = startVM()
	})
	AfterEach(func() {
		vm.Destroy(func(vm VM) {})
	})

	It("eventually boots", func() {
		vm.EventuallyConnects(1200)
	})
})
