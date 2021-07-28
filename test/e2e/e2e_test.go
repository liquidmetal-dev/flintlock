// +build e2e

package e2e_test

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/gruntwork-io/terratest/modules/environment"
	"github.com/gruntwork-io/terratest/modules/retry"
	"github.com/gruntwork-io/terratest/modules/ssh"
	"github.com/gruntwork-io/terratest/modules/terraform"
	test_structure "github.com/gruntwork-io/terratest/modules/test-structure"
)

func TestBasicScenario(t *testing.T) {
	RegisterTestingT(t)

	terraDir := "./infra"

	defer test_structure.RunTestStage(t, "teardown", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, terraDir)
		terraform.Destroy(t, terraformOptions)
		test_structure.CleanupTestDataFolder(t, terraDir)
	})

	test_structure.RunTestStage(t, "setup", func() {
		terraformOptions := configureTerraformOptionsAndSave(t, terraDir)

		// This will run `terraform init` and `terraform apply` and fail the test if there are any errors
		terraform.InitAndApply(t, terraformOptions)
	})

	test_structure.RunTestStage(t, "validate", func() {
		terraformOptions := test_structure.LoadTerraformOptions(t, terraDir)
		keyPair := test_structure.LoadSshKeyPair(t, terraDir)

		testSSHToHost(t, terraformOptions, keyPair)
	})
}

func testSSHToHost(t *testing.T, terraformOptions *terraform.Options, keyPair *ssh.KeyPair) {
	publicIPAddress := terraform.Output(t, terraformOptions, "public_ip")
	Expect(publicIPAddress).NotTo(BeEmpty())

	publicHost := ssh.Host{
		Hostname:    publicIPAddress,
		SshKeyPair:  keyPair,
		SshUserName: "root",
	}

	maxRetries := 30
	timeBetweenRetries := 5 * time.Second
	description := fmt.Sprintf("SSH to host %s", publicIPAddress)
	command := "cat /tmp/metadata"

	retry.DoWithRetry(t, description, maxRetries, timeBetweenRetries, func() (string, error) {
		actualText, err := ssh.CheckSshCommandE(t, publicHost, command)
		if err != nil {
			return "", err
		}

		Expect(actualText).ToNot(BeEmpty())
		return "", nil
	})
}

func configureTerraformOptionsAndSave(t *testing.T, dir string) *terraform.Options {
	// NOTE: ioutil has been used in places to write the file so that the contents aren't output to the logs
	// Generate and save the options the kaypair
	keyPair := ssh.GenerateRSAKeyPair(t, 4096)
	test_structure.SaveSshKeyPair(t, dir, keyPair)
	publicKeyPath := test_structure.FormatTestDataPath(dir, "id_rsa.pub")
	ioutil.WriteFile(publicKeyPath, []byte(keyPair.PublicKey), os.ModePerm)
	privateKeyPath := test_structure.FormatTestDataPath(dir, "id_rsa")
	ioutil.WriteFile(privateKeyPath, []byte(keyPair.PrivateKey), os.ModePerm)

	// Save the API key
	apiToken := environment.GetFirstNonEmptyEnvVarOrFatal(t, []string{"METAL_AUTH_TOKEN"})
	apiTokenPath := test_structure.FormatTestDataPath(dir, "api_token")
	ioutil.WriteFile(apiTokenPath, []byte(apiToken), os.ModePerm)

	// Get absolute paths
	apiTokenPathAbs, _ := filepath.Abs(apiTokenPath)
	privateKeyPathAbs, _ := filepath.Abs(privateKeyPath)
	publicKeyPathAbs, _ := filepath.Abs(publicKeyPath)

	// Create and save the options
	terraformOptions := terraform.WithDefaultRetryableErrors(t, &terraform.Options{
		TerraformDir: dir,
		Vars: map[string]interface{}{
			"ssh_user":         "root",
			"auth_token_path":  apiTokenPathAbs,
			"public_key_path":  publicKeyPathAbs,
			"private_key_path": privateKeyPathAbs,
		},
		NoColor: true,
	})
	test_structure.SaveTerraformOptions(t, dir, terraformOptions)

	return terraformOptions
}
