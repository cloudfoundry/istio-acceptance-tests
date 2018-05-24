package bookinfo_demo

import (
	"os"
	"testing"
	"time"

	"code.cloudfoundry.org/istio-acceptance-tests/config"
	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/workflowhelpers"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
	"github.com/sclevine/agouti"
)

var (
	agoutiDriver   *agouti.WebDriver
	c              config.Config
	TestSetup      *workflowhelpers.ReproducibleTestSuiteSetup
	defaultTimeout = 120 * time.Second
)

func TestBookinfoDemo(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "BookinfoDemo Suite")
}

var _ = SynchronizedBeforeSuite(func() []byte {
	var err error
	configPath := os.Getenv("CONFIG")
	Expect(configPath).NotTo(BeEmpty())

	c, err = config.NewConfig(configPath)
	Expect(err).ToNot(HaveOccurred())
	Expect(c.Validate()).To(Succeed())

	_, ok := os.LookupEnv("INTERNAL_DOMAIN")
	if !ok {
		os.Setenv("INTERNAL_DOMAIN", c.CFInternalAppsDomain)
	}
	_, ok = os.LookupEnv("API_DOMAIN")
	if !ok {
		os.Setenv("API_DOMAIN", c.IstioDomain)
	}

	workflowhelpers.AsUser(TestSetup.AdminUserContext(), defaultTimeout, func() {
		Expect(cf.Cf("enable-feature-flag", "diego_docker").Wait(defaultTimeout)).To(Exit(0))
	})
	TestSetup = workflowhelpers.NewTestSuiteSetup(c)
	TestSetup.Setup()

	Expect(cf.Cf("push", "productpage", "-o", c.ProductPageDockerWithTag, "-d", c.IstioDomain).Wait(defaultTimeout)).To(Exit(0))
	Expect(cf.Cf("push", "ratings", "-o", c.RatingsDockerWithTag, "-d", c.CFInternalAppsDomain).Wait(defaultTimeout)).To(Exit(0))
	Expect(cf.Cf("push", "reviews", "-o", c.ReviewsDockerWithTag, "-d", c.CFInternalAppsDomain, "-u", "none").Wait(defaultTimeout)).To(Exit(0))
	Expect(cf.Cf("push", "details", "-o", c.DetailsDockerWithTag, "-d", c.CFInternalAppsDomain).Wait(defaultTimeout)).To(Exit(0))
	Expect(cf.Cf("set-env", "productpage", "SERVICES_DOMAIN", c.CFInternalAppsDomain).Wait(defaultTimeout)).To(Exit(0))
	Expect(cf.Cf("restage", "productpage").Wait(defaultTimeout)).To(Exit(0))
	Expect(cf.Cf("set-env", "reviews", "SERVICES_DOMAIN", c.CFInternalAppsDomain).Wait(defaultTimeout)).To(Exit(0))
	Expect(cf.Cf("restage", "reviews").Wait(defaultTimeout)).To(Exit(0))

	workflowhelpers.AsUser(TestSetup.AdminUserContext(), defaultTimeout, func() {
		Expect(cf.Cf("target", "-o", TestSetup.TestSpace.OrganizationName(), "-s", TestSetup.TestSpace.SpaceName()).Wait(defaultTimeout)).To(Exit(0))
		Expect(cf.Cf("add-network-policy", "productpage", "--destination-app", "details", "--protocol", "tcp", "--port", "9080").Wait(defaultTimeout)).To(Exit(0))
		Expect(cf.Cf("add-network-policy", "productpage", "--destination-app", "reviews", "--protocol", "tcp", "--port", "9080").Wait(defaultTimeout)).To(Exit(0))
		Expect(cf.Cf("add-network-policy", "reviews", "--destination-app", "ratings", "--protocol", "tcp", "--port", "9080").Wait(defaultTimeout)).To(Exit(0))
	})
	return []byte{}
}, func(data []byte) {
	agoutiDriver = agouti.ChromeDriver(
		agouti.ChromeOptions("args", []string{
			"--headless",
			"--disable-gpu",
			"--allow-insecure-localhost",
			"--no-sandbox",
		}),
	)
	Expect(agoutiDriver.Start()).To(Succeed())
})

var _ = SynchronizedAfterSuite(func() {
	if TestSetup != nil {
		TestSetup.Teardown()
	}
}, func() {
	Expect(agoutiDriver.Stop()).To(Succeed())
})
