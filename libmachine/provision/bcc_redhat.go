package provision

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/docker/machine/libmachine/drivers"
)

var (
	bccPackageListTemplate = `[docker]
name=Docker Stable Repository
baseurl=https://yum.dockerproject.org/repo/main/{{.OsRelease}}/{{.OsReleaseVersion}}
priority=1
enabled=1
gpgkey=https://yum.dockerproject.org/gpg
`
	bccEngineConfigTemplate = `[Unit]
Description=Docker Application Container Engine
After=network.target docker.socket
Requires=docker.socket

[Service]
ExecStart=/usr/bin/docker daemon -H tcp://0.0.0.0:{{.DockerPort}} -H unix:///var/run/docker.sock --storage-driver {{.EngineOptions.StorageDriver}} --tlsverify --tlscacert {{.AuthOptions.CaCertRemotePath}} --tlscert {{.AuthOptions.ServerCertRemotePath}} --tlskey {{.AuthOptions.ServerKeyRemotePath}} {{ range .EngineOptions.Labels }}--label {{.}} {{ end }}{{ range .EngineOptions.InsecureRegistry }}--insecure-registry {{.}} {{ end }}{{ range .EngineOptions.RegistryMirror }}--registry-mirror {{.}} {{ end }}{{ range .EngineOptions.ArbitraryFlags }}--{{.}} {{ end }}
MountFlags=slave
LimitNOFILE=1048576
LimitNPROC=1048576
LimitCORE=infinity
Environment={{range .EngineOptions.Env}}{{ printf "%q" . }} {{end}}
`
)

func init() {
	Register("RedHat", &RegisteredProvisioner{
		New: func(d drivers.Driver) Provisioner {
			return NewBCCRedHatProvisioner("bcc-rhel", d)
		},
	})
}

func NewBCCRedHatProvisioner(osReleaseID string, d drivers.Driver) *BCCRedHatProvisioner {
	redhatProvisioner := NewRedHatProvisioner(osReleaseID, d)
	return &BCCRedHatProvisioner{
		*redhatProvisioner,
	}
}

type BCCRedHatProvisioner struct {
	RedHatProvisioner
}

func (provisioner *BCCRedHatProvisioner) String() string {
	return "bcc-redhat"
}

// noop since BCC Redhat already has correct package list
func (provisioner *BCCRedHatProvisioner) ConfigurePackageList() error {
	return nil
}

func (provisioner *BCCRedHatProvisioner) installDocker() error {
	if _, err := provisioner.SSHCommand("sudo yum install -y docker"); err != nil {
		return err
	}

	return nil
}

func (provisioner *BCCRedHatProvisioner) GenerateDockerOptions(dockerPort int) (*DockerOptions, error) {
	var (
		engineCfg  bytes.Buffer
		configPath = provisioner.DaemonOptionsFile
	)

	driverNameLabel := fmt.Sprintf("provider=%s", provisioner.Driver.DriverName())
	provisioner.EngineOptions.Labels = append(provisioner.EngineOptions.Labels, driverNameLabel)

	// systemd / redhat will not load options if they are on newlines
	// instead, it just continues with a different set of options; yeah...
	t, err := template.New("engineConfig").Parse(engineConfigTemplate)
	if err != nil {
		return nil, err
	}

	engineConfigContext := EngineConfigContext{
		DockerPort:       dockerPort,
		AuthOptions:      provisioner.AuthOptions,
		EngineOptions:    provisioner.EngineOptions,
		DockerOptionsDir: provisioner.DockerOptionsDir,
	}

	t.Execute(&engineCfg, engineConfigContext)

	daemonOptsDir := configPath
	return &DockerOptions{
		EngineOptions:     engineCfg.String(),
		EngineOptionsPath: daemonOptsDir,
	}, nil
}
