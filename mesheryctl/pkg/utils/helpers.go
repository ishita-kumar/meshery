package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

const (
	dockerComposeWebURL         = "https://api.github.com/repos/docker/compose/releases/latest"
	defaultDockerComposeVersion = "1.24.1/docker-compose"
	dockerComposeBinaryURL      = "https://github.com/docker/compose/releases/download/"
	dockerComposeBinary         = "/usr/local/bin/docker-compose"

	// Usage URLs
	rootUsageURL   = "https://meshery.layer5.io/docs/guides/mesheryctl#global-commands-and-flags"
	perfUsageURL   = "https://meshery.layer5.io/docs/guides/mesheryctl#performance-management"
	systemUsageURL = "https://meshery.layer5.io/docs/guides/mesheryctl#meshery-lifecycle-management"
	meshUsageURL   = "https://meshery.layer5.io/docs/guides/mesheryctl#service-mesh-lifecycle-management"
)

type cmdType string

const (
	cmdRoot   cmdType = "root"
	cmdPerf   cmdType = "perf"
	cmdMesh   cmdType = "mesh"
	cmdSystem cmdType = "system"
)

var (
	// ResetFlag indicates if a reset is required
	ResetFlag bool
	// MesheryFolder is the default relative location of the meshery config
	// related configuration files.
	MesheryFolder = ".meshery"
	// DockerComposeFile is the default location within the MesheryFolder
	// where the docker compose file is located?
	DockerComposeFile = "meshery.yaml"
	// AuthConfigFile is the location of the auth file for performing perf testing
	AuthConfigFile = "auth.json"
)

// SafeClose is a helper function help to close the io
func SafeClose(co io.Closer) {
	if cerr := co.Close(); cerr != nil {
		log.Error(cerr)
	}
}

// DownloadFile from url and save to configured file location
func DownloadFile(filepath string, url string) error {
	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return errors.Wrapf(err, "failed to make GET request to %s", url)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return errors.Wrapf(err, "failed to create file %s", filepath)
	}
	defer func() {
		_ = out.Close()
	}()
	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return errors.Wrap(err, "failed to copy response body")
	}

	return nil
}

func prereq() ([]byte, []byte, error) {
	ostype, err := exec.Command("uname", "-s").Output()
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not find os type")
	}

	osarch, err := exec.Command("uname", "-m").Output()
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not find os arch type")
	}

	return ostype, osarch, nil
}

// SetFileLocation to set absolute path
func SetFileLocation() error {
	// Find home directory.
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "failed to get users home directory")
	}
	MesheryFolder = path.Join(home, MesheryFolder)
	DockerComposeFile = path.Join(MesheryFolder, DockerComposeFile)
	AuthConfigFile = path.Join(MesheryFolder, AuthConfigFile)
	return nil
}

//PreReqCheck prerequisites check
func PreReqCheck() error {
	//Check for installed docker-compose on client system
	if err := exec.Command("docker-compose", "-v").Run(); err != nil {
		log.Info("Docker-Compose is not installed")
		//No auto installation of Docker-compose for windows
		if runtime.GOOS == "windows" {
			return errors.Wrap(err, "please install docker-compose")
		}
		err = installprereq()
		if err != nil {
			return errors.Wrap(err, "failed to install prerequisites")
		}
	}
	return nil
}

func installprereq() error {
	log.Info("Attempting Docker-Compose installation...")
	ostype, osarch, err := prereq()
	if err != nil {
		return errors.Wrap(err, "failed to get prerequisites")
	}

	osdetails := strings.TrimRight(string(ostype), "\r\n") + "-" + strings.TrimRight(string(osarch), "\r\n")

	dockerComposeBinaryURL := dockerComposeBinaryURL
	//checks for the latest docker-compose
	resp, err := http.Get(dockerComposeWebURL)
	if err != nil {
		dockerComposeBinaryURL = dockerComposeBinaryURL + defaultDockerComposeVersion
	} else {
		var dat map[string]interface{}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return errors.Wrap(err, "failed to read response body")
		}
		if err := json.Unmarshal(body, &dat); err != nil {
			return errors.Wrap(err, "failed to unmarshal json into object")
		}
		num := dat["tag_name"]
		dockerComposeBinaryURL = fmt.Sprintf(dockerComposeBinaryURL+"%v/docker-compose", num)
	}
	dockerComposeBinaryURL = dockerComposeBinaryURL + "-" + osdetails
	if err := DownloadFile(dockerComposeBinary, dockerComposeBinaryURL); err != nil {
		return errors.Wrapf(err, "failed to download %s from %s", dockerComposeBinary, dockerComposeBinaryURL)
	}
	if err := exec.Command("chmod", "+x", dockerComposeBinary).Run(); err != nil {
		return errors.Wrap(err, "failed to execute command")
	}
	log.Info("Prerequisite Docker Compose is installed.")
	return nil
}

// IsMesheryRunning checks if the meshery server containers are up and running
func IsMesheryRunning() bool {
	op, err := exec.Command("docker-compose", "-f", DockerComposeFile, "ps").Output()
	if err != nil {
		return false
	}
	return strings.Contains(string(op), "meshery")
}

// RootError returns a formatted error message with a link to 'root' command usage page at
// in addition to the error message
func RootError(msg string) string {
	return formatError(msg, cmdRoot)
}

// PerfError returns a formatted error message with a link to 'perf' command usage page at
// in addition to the error message
func PerfError(msg string) string {
	return formatError(msg, cmdPerf)
}

// SystemError returns a formatted error message with a link to 'system' command usage page
// in addition to the error message
func SystemError(msg string) string {
	return formatError(msg, cmdSystem)
}

// MeshError returns a formatted error message with a link to 'mesh' command usage page
// in addition to the error message
//func MeshError(msg string) string {
//	return formatError(msg, cmdMesh)
//}

// formatError returns a formatted error message with a link to the meshery command URL
func formatError(msg string, cmd cmdType) string {
	switch cmd {
	case cmdRoot:
		return fmt.Sprintf("%s\nSee %s for usage details\n", msg, rootUsageURL)
	case cmdPerf:
		return fmt.Sprintf("%s\nSee %s for usage details\n", msg, perfUsageURL)
	case cmdMesh:
		return fmt.Sprintf("%s\nSee %s for usage details\n", msg, meshUsageURL)
	case cmdSystem:
		return fmt.Sprintf("%s\nSee %s for usage details\n", msg, systemUsageURL)
	}
	return fmt.Sprintf("%s\n", msg)
}

// IsValidSubcommand checks if the passed subcommand is supported by the parent command
func IsValidSubcommand(available []*cobra.Command, sub string) bool {
	for _, s := range available {
		if sub == s.CalledAs() {
			return true
		}
	}
	return false
}
