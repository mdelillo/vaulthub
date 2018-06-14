package acceptance_test

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Acceptance", func() {
	var vaulthubBin string
	var vault func(args ...string) string
	var vaulthubAddress string

	BeforeSuite(func() {
		rand.Seed(time.Now().UnixNano())

		_, err := exec.LookPath("vault")
		Expect(err).NotTo(HaveOccurred(), "could not find `vault` executable")

		vaulthubBin, err = gexec.Build("github.com/mdelillo/vaulthub")
		Expect(err).NotTo(HaveOccurred())
		time.Sleep(time.Second)
	})

	AfterSuite(func() {
		gexec.CleanupBuildArtifacts()
	})

	BeforeEach(func() {
		vaultAddress, vaultToken := startVault()
		vault = func(args ...string) string {
			cmd := exec.Command("vault", args...)
			cmd.Env = append(os.Environ(), "VAULT_ADDR="+vaultAddress, "VAULT_TOKEN="+vaultToken)
			session, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			ExpectWithOffset(1, err).NotTo(HaveOccurred())
			EventuallyWithOffset(1, session).Should(gexec.Exit(0))
			return string(session.Out.Contents())
		}
		time.Sleep(time.Second)

		vaulthubAddress = startVaulthub(vaulthubBin, vaultAddress, vaultToken)
		time.Sleep(time.Second)
	})

	AfterEach(func() {
		gexec.KillAndWait()
	})

	Context("getting credentials", func() {
		It("gets a password from vault", func() {
			vault("kv", "put", "/secret/some-secret", "value=some-value")

			resp, err := http.Get(vaulthubAddress + "/api/v1/data/some-secret")
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(body)).To(Equal("some-value"))
		})
	})

	Context("setting credentials", func() {
		It("sets a password in vault", func() {
			value := randomString()
			_, err := http.Post(
				vaulthubAddress+"/api/v1/data/some-secret",
				"application/json",
				strings.NewReader(fmt.Sprintf(`{"value":"%s"}`, value)),
			)
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(time.Second)

			fmt.Println("SECRETS:\n" + vault("kv", "list", "secret"))

			actualValue := vault("kv", "get", "-field", "value", "/secret/some-secret")
			Expect(actualValue).To(Equal(value))
		})
	})

	XContext("generating credentials", func() {
		It("generates a password in vault", func() {
		})
	})
})

func startVaulthub(vaulthub, vaultAddress, vaultToken string) string {
	address := getFreeAddress()
	cmd := exec.Command(vaulthub,
		"--address", address,
		"--vault-address", vaultAddress,
		"--vault-token", vaultToken,
	)
	_, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return "http://" + address
}

func startVault() (string, string) {
	address := getFreeAddress()
	token := randomString()
	cmd := exec.Command("vault", "server", "-dev",
		"-dev-listen-address", address,
		"-dev-root-token-id", token,
	)
	_, err := gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())
	return "http://" + address, token
}

func getFreeAddress() string {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	Expect(err).NotTo(HaveOccurred())
	defer listener.Close()
	return listener.Addr().String()
}

func randomString() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, rand.Intn(25)+25)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
