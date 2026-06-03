package sysutils

import (
	"os"
	"strings"
	"time"
)

// SystemUpdateRepos configures APT repositories
func SystemUpdateRepos() error {
	Info("Configuring APT repositories...")

	out, _ := RunCommand(10*time.Second, "dpkg", "-l")
	if !strings.Contains(out, "multimedia-keyring") {
		// Download and install keyring
		tmpDir, err := os.MkdirTemp("", "keyring")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		keyring := "deb-multimedia-keyring_2024.9.1_all.deb"
		url := "https://www.deb-multimedia.org/pool/main/d/deb-multimedia-keyring/" + keyring

		_, err = RunCommand(20*time.Second, "wget", "--https-only", "--secure-protocol=TLSv1_2", "-q", url, "-O", tmpDir+"/"+keyring)
		if err == nil {
			RunCommand(20*time.Second, "dpkg", "-i", tmpDir+"/"+keyring)
		}
	}

	RunCommand(10*time.Minute, "apt", "-y", "modernize-sources")

	aptFile := "/etc/apt/sources.list.d/debian.sources"
	debContent := `Types: deb
URIs: https://deb.debian.org/debian/
Suites: testing testing-updates experimental
Components: main contrib non-free non-free-firmware
Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg

Types: deb
URIs: http://security.debian.org/debian-security/
Suites: testing-security
Components: main contrib non-free non-free-firmware
Signed-By: /usr/share/keyrings/debian-archive-keyring.gpg
`
	os.WriteFile(aptFile, []byte(debContent), 0644)

	mmFile := "/etc/apt/sources.list.d/multimedia.sources"
	mmContent := `Types: deb
URIs: https://www.deb-multimedia.org/
Suites: testing
Components: main non-free
Signed-By: /usr/share/keyrings/deb-multimedia-keyring.pgp
`
	os.WriteFile(mmFile, []byte(mmContent), 0644)

	RunCommand(10*time.Minute, "apt-get", "update", "-y")
	RunCommand(10*time.Minute, "apt-get", "install", "-y", "aptitude", "apt", "dpkg")

	Success("Repos updated.")
	return nil
}
