package operators

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	deploymentWait     = 5 * time.Minute
	ingressNamespace   = "openshift-ingress"
	rolloutWaitTimeout = "--request-timeout=5m"
)

var _ = g.Describe("[Feature:Platform] Ingress operator should", func() {
	defer g.GinkgoRecover()
	var (
		oc                   = exutil.NewCLI("cluster-ingress-cert", exutil.KubeConfigPath())
		configPath           = exutil.FixturePath("testdata", "operators", "clusteringress", "config.yaml")
		updatedConfigPath    = exutil.FixturePath("testdata", "operators", "clusteringress", "config-with-updated-secret.yaml")
		certSecretPath       = exutil.FixturePath("testdata", "operators", "clusteringress", "secret.yaml")
		customCertConfigPath = exutil.FixturePath("testdata", "operators", "clusteringress", "config-with-custom-cert.yaml")
		customCertSecretPath = exutil.FixturePath("testdata", "operators", "clusteringress", "custom-cert-secret.yaml")
	)

	g.It("configure the default router with a generated secret", func() {
		g.By("waiting for default router daemonset")
		err := checkIngressCreatedDaemonSet(oc, "router-default", "router-certs-default")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("configure custom routers with custom certificates", func() {
		g.By("creating custom certificate secret")
		output, err := oc.Run("create").Args("-f", customCertSecretPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("oc create -f %s : %v", customCertSecretPath, output)

		g.By("creating ingress config with a custom certificate")
		output, err = oc.Run("create").Args("-f", customCertConfigPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("oc create -f %s : %v", customCertConfigPath, output)

		g.By("waiting for custom router daemonset")
		err = checkIngressCreatedDaemonSet(oc, "router-custom-cert-test", "custom-cert-secret")
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("allow router certificates to be updated", func() {
		g.By("creating certificate secret")
		output, err := oc.Run("create").Args("-f", certSecretPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating ingress config")
		output, err = oc.Run("create").Args("-f", configPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("oc create -f %s : %v", configPath, output)

		g.By("waiting for router daemonset")
		err = checkIngressCreatedDaemonSet(oc, "router-checker", "router-certs-checker")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("updating ingress config")
		apply, err := oc.Run("apply").Args("-f", updatedConfigPath).Output()

		g.By("waiting for new rollout")
		rollout, err := oc.Run("rollout").Args("status", "ds/router-checker", "-n openshift-ingress", rolloutWaitTimeout).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("oc rollout status ds/router-checker -n openshift-ingress: %v", rollout)

		g.By("waiting for router daemonset secret to be updated")
		err = checkIngressCreatedDaemonSet(oc, "router-checker", "hush-hush")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("reverting back the ingress config")
		apply, err = oc.Run("apply").Args("-f", configPath).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("oc apply -f %s : %v", configPath, apply)

		g.By("waiting for new rollout")
		rollout, err = oc.Run("rollout").Args("status", "ds/router-checker", "-n openshift-ingress", rolloutWaitTimeout).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("oc rollout status ds/router-checker -n openshift-ingress: %v", rollout)

		g.By("waiting for router daemonset secret to be reverted")
		err = checkIngressCreatedDaemonSet(oc, "router-checker", "router-certs-checker")
		o.Expect(err).NotTo(o.HaveOccurred())

	})
})

func checkIngressCreatedDaemonSet(oc *exutil.CLI, name, secretName string) error {
	var dsGetErr error
	err := wait.PollImmediate(3*time.Second, deploymentWait, func() (bool, error) {
		ds, err := oc.KubeClient().ExtensionsV1beta1().DaemonSets(ingressNamespace).Get(name, metav1.GetOptions{})
		if err != nil {
			dsGetErr = err
			e2e.Logf("Unable to find router %s daemonset config: %v", name, err)
			return false, nil
		}

		dsGetErr = nil
		volumeSecretName := ds.Spec.Template.Spec.Volumes[0].Secret.SecretName
		if volumeSecretName != secretName {
			msg := fmt.Sprintf("volume secret name %s does not match expectation %s", volumeSecretName, secretName)
			e2e.Logf(msg)
			return true, fmt.Errorf(msg)
		}
		return true, nil
	})
	if dsGetErr != nil {
		return dsGetErr
	}

	return err
}
