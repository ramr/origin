package builds

import (
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/util/wait"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:Builds][Conformance] remove all builds when build configuration is removed", func() {
	defer g.GinkgoRecover()
	var (
		buildFixture = exutil.FixturePath("testdata", "builds", "test-build.yaml")
		oc           = exutil.NewCLI("cli-remove-build", exutil.KubeConfigPath())
	)

	g.Context("", func() {
		g.BeforeEach(func() {
			exutil.PreTestDump()
		})

		g.JustBeforeEach(func() {
			g.By("waiting for default service account")
			err := exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "default")
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By("waiting for builder service account")
			err = exutil.WaitForServiceAccount(oc.KubeClient().Core().ServiceAccounts(oc.Namespace()), "builder")
			o.Expect(err).NotTo(o.HaveOccurred())

			oc.Run("create").Args("-f", buildFixture).Execute()
		})

		g.AfterEach(func() {
			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpConfigMapStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.Describe("oc delete buildconfig", func() {
			g.It("should start builds and delete the buildconfig", func() {
				var (
					err    error
					builds [4]string
				)

				g.By("starting multiple builds")
				for i := range builds {
					stdout, _, err := exutil.StartBuild(oc, "sample-build", "-o=name")
					o.Expect(err).NotTo(o.HaveOccurred())
					builds[i] = stdout
				}

				g.By("deleting the buildconfig")
				err = oc.Run("delete").Args("bc/sample-build").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				g.By("waiting for builds to clear")
				err = wait.Poll(3*time.Second, 3*time.Minute, func() (bool, error) {
					out, err := oc.Run("get").Args("builds").Output()
					o.Expect(err).NotTo(o.HaveOccurred())
					if out == "No resources found." {
						return true, nil
					}
					return false, nil
				})
				if err == wait.ErrWaitTimeout {
					g.Fail("timed out waiting for builds to clear")
				}
			})

		})
	})
})
