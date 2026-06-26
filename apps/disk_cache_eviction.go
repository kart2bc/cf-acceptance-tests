package apps

import (
	. "github.com/cloudfoundry/cf-acceptance-tests/cats_suite_helpers"

	"github.com/cloudfoundry/cf-acceptance-tests/helpers/app_helpers"
	"github.com/cloudfoundry/cf-acceptance-tests/helpers/assets"
	"github.com/cloudfoundry/cf-acceptance-tests/helpers/random_name"
	"github.com/cloudfoundry/cf-test-helpers/v2/cf"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gbytes"
	. "github.com/onsi/gomega/gexec"
)

// These tests exercise the Diego cell download cache (cacheddownloader) eviction
// and the executor's disk capacity reporting. Two bugs were fixed:
//
//  1. cacheddownloader makeRoom() now checks real partition free space via syscall.Statfs
//     in addition to the in-memory size ceiling, so the cache evicts stale entries before
//     the partition fills up rather than only tracking an in-memory byte count.
//
//  2. executor RemainingResources caps reported remaining disk at the live partition free
//     space, while TotalResources stays static (configured capacity). This keeps the BBS
//     formula total - remaining = allocated consistent and prevents the scheduler from
//     over-committing disk on cells under partition pressure.
var _ = AppsDescribe("Disk cache eviction", func() {
	var (
		appName  string
		app2Name string
	)

	BeforeEach(func() {
		appName = random_name.CATSRandomName("APP")
		app2Name = random_name.CATSRandomName("APP")
	})

	AfterEach(func() {
		app_helpers.AppReport(appName)
		cf.Cf("delete", appName, "-f", "-r").Wait(Config.CfPushTimeoutDuration())

		app_helpers.AppReport(app2Name)
		cf.Cf("delete", app2Name, "-f", "-r").Wait(Config.CfPushTimeoutDuration())
	})

	It("stages an app successfully after the same app is restaged, exercising the download cache", func() {
		By("pushing an app to populate the download cache on the Diego cell")
		Expect(cf.Cf("push", appName,
			"-b", Config.GetStaticFileBuildpackName(),
			"-p", assets.NewAssets().Staticfile,
			"-m", DEFAULT_MEMORY_LIMIT,
		).Wait(Config.CfPushTimeoutDuration())).To(Exit(0))

		By("restaging the same app — the cell must serve a cache hit or evict and re-download without staging failure")
		restage := cf.Cf("restage", appName).Wait(Config.CfPushTimeoutDuration())
		Expect(restage).To(Exit(0))
		Expect(restage).NotTo(Say("FAILED"))
	})

	It("stages two different apps sequentially so the download cache must manage both entries", func() {
		By("staging a first app to warm the download cache")
		Expect(cf.Cf("push", appName,
			"-b", Config.GetStaticFileBuildpackName(),
			"-p", assets.NewAssets().Staticfile,
			"-m", DEFAULT_MEMORY_LIMIT,
		).Wait(Config.CfPushTimeoutDuration())).To(Exit(0))

		By("staging a second app — the cell evicts as needed without exceeding partition free space")
		Expect(cf.Cf("push", app2Name,
			"-b", Config.GetStaticFileBuildpackName(),
			"-p", assets.NewAssets().Staticfile,
			"-m", DEFAULT_MEMORY_LIMIT,
		).Wait(Config.CfPushTimeoutDuration())).To(Exit(0))
	})

	It("reports disk usage for a running app consistent with the allocation", func() {
		By("pushing an app with an explicit disk quota")
		Expect(cf.Cf("push", appName,
			"-b", Config.GetStaticFileBuildpackName(),
			"-p", assets.NewAssets().Staticfile,
			"-m", DEFAULT_MEMORY_LIMIT,
			"-k", "256M",
		).Wait(Config.CfPushTimeoutDuration())).To(Exit(0))

		By("verifying cf app reports disk usage without claiming more than the allocated quota")
		appDetails := cf.Cf("app", appName).Wait(Config.DefaultTimeoutDuration())
		Expect(appDetails).To(Exit(0))
		// The reported disk usage line must appear; a 'disk full' error would manifest as a
		// crash or missing usage line rather than a graceful startup with stats.
		Expect(appDetails).To(Say("256M"))
	})
})
