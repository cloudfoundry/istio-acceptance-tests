package routing_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/generator"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

type AppResponse struct {
	Greeting     string `json:"greeting"`
	InstanceGUID string `json:"instance_guid"`
}

var _ = Describe("Context Paths", func() {
	var (
		domain            string
		app               string
		hostname          string
		contextPath       string
		helloRoutingAsset = "../assets/hello-golang"
	)

	BeforeEach(func() {
		domain = istioDomain()
		hostname = generator.PrefixedRandomName("IATS", "host")
		contextPath = "/nothing/matters"

		app = generator.PrefixedRandomName("IATS", "APP")
		Expect(cf.Cf("push", app,
			"-p", helloRoutingAsset,
			"-f", fmt.Sprintf("%s/manifest.yml", helloRoutingAsset),
			"-n", hostname,
			"-d", domain,
			"--route-path", contextPath,
			"-i", "1").Wait(defaultTimeout)).To(Exit(0))
	})

	Context("when using a context path", func() {
		It("should route to the appropriate app", func() {
			baseURL := fmt.Sprintf("http://%s.%s", hostname, domain)
			contextPathURL := fmt.Sprintf("http://%s.%s%s", hostname, domain, contextPath)

			Consistently(func() (int, error) {
				return getStatusCode(baseURL)
			}, "15s").Should(Equal(http.StatusNotFound))

			Eventually(func() (int, error) {
				return getStatusCode(contextPathURL)
			}, defaultTimeout, time.Second).Should(Equal(http.StatusOK))

			res, err := http.Get(contextPathURL)
			Expect(err).ToNot(HaveOccurred())

			body, err := ioutil.ReadAll(res.Body)
			Expect(err).ToNot(HaveOccurred())

			var appResponse AppResponse
			err = json.Unmarshal(body, &appResponse)
			Expect(err).ToNot(HaveOccurred())

			Expect(appResponse.Greeting).To(Equal("hello"))
		})
	})

	Context("when manipulating a route with a context path", func() {
		It("routes continues to route", func() {
			By("unmapping the route")
			contextPathURL := fmt.Sprintf("http://%s.%s%s", hostname, domain, contextPath)

			Eventually(func() (int, error) {
				return getStatusCode(contextPathURL)
			}, defaultTimeout, time.Second).Should(Equal(http.StatusOK))

			Expect(cf.Cf("unmap-route", app, domain,
				"--hostname", hostname,
				"--path", contextPath).Wait(defaultTimeout)).To(Exit(0))

			Eventually(func() (int, error) {
				return getStatusCode(contextPathURL)
			}, defaultTimeout, time.Second).Should(Equal(http.StatusNotFound))

			By("deleting the route")
			Expect(cf.Cf("delete-route", domain,
				"-f",
				"--hostname", hostname,
				"--path", contextPath).Wait(defaultTimeout)).To(Exit(0))

			Eventually(func() (int, error) {
				return getStatusCode(contextPathURL)
			}, defaultTimeout, time.Second).Should(Equal(http.StatusNotFound))

			By("verifying context path still routes to best match")
			Expect(cf.Cf("map-route", app, domain,
				"--hostname", hostname).Wait(defaultTimeout)).To(Exit(0))

			Eventually(func() (int, error) {
				return getStatusCode(contextPathURL)
			}, defaultTimeout, time.Second).Should(Equal(http.StatusOK))
		})
	})

	Context("when mapping multiple routes to the same app", func() {
		It("routes successfully", func() {
			By("mapping a second context path")
			contextPathTwo := "/nothing/matters/again"
			Expect(cf.Cf("map-route", app, domain,
				"--hostname", hostname,
				"--path", contextPathTwo).Wait(defaultTimeout)).To(Exit(0))

			Eventually(func() (int, error) {
				return getStatusCode(fmt.Sprintf("http://%s.%s%s", hostname, domain, contextPathTwo))
			}, defaultTimeout, time.Second).Should(Equal(http.StatusOK))

			By("mapping a second hostname")
			otherHostname := generator.PrefixedRandomName("IATS", "otherhost")
			Expect(cf.Cf("map-route", app, domain,
				"--hostname", otherHostname).Wait(defaultTimeout)).To(Exit(0))

			Eventually(func() (int, error) {
				return getStatusCode(fmt.Sprintf("http://%s.%s", otherHostname, domain))
			}, defaultTimeout, time.Second).Should(Equal(http.StatusOK))
		})
	})

	Context("when multiple apps are pushed", func() {
		var (
			otherContextPath string
		)

		BeforeEach(func() {
			otherContextPath = "/everything/matters"

			app = generator.PrefixedRandomName("IATS", "APP")
			Expect(cf.Cf("push", app,
				"-p", helloRoutingAsset,
				"-f", fmt.Sprintf("%s/manifest.yml", helloRoutingAsset),
				"-n", hostname,
				"-d", domain,
				"--route-path", otherContextPath,
				"-i", "1").Wait(defaultTimeout)).To(Exit(0))
		})

		It("routes succesfully to different apps with mapped to the same hostname", func() {
			var instanceGuid string
			Eventually(func() (int, error) {
				res, err := http.Get(fmt.Sprintf("http://%s.%s%s", hostname, domain, otherContextPath))
				if err != nil {
					return 0, nil
				}

				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return 0, nil
				}

				var appResponse AppResponse
				err = json.Unmarshal(body, &appResponse)
				if err != nil {
					return 0, nil
				}
				instanceGuid = appResponse.InstanceGUID

				return res.StatusCode, nil
			}, defaultTimeout, time.Second).Should(Equal(http.StatusOK))

			Consistently(func() (bool, error) {
				res, err := http.Get(fmt.Sprintf("http://%s.%s%s", hostname, domain, contextPath))
				if err != nil {
					return false, err
				}

				body, err := ioutil.ReadAll(res.Body)
				if err != nil {
					return false, err
				}

				var appResponse AppResponse
				err = json.Unmarshal(body, &appResponse)
				if err != nil {
					return false, err
				}

				return instanceGuid != appResponse.InstanceGUID, nil
			}, "15s", time.Second).Should(BeTrue())
		})
	})
})
