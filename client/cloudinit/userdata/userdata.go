package userdata

type UserData struct {
	HostName        string      `yaml:"hostname,omitempty"`
	Fqdn            string      `yaml:"fqdn,omitempty"`
	Users           []User      `yaml:"users,omitempty"`
	SSHPasswordAuth *bool       `yaml:"ssh_pwauth,omitempty"`
	DisableRoot     *bool       `yaml:"disable_root,omitempty"`
	PackageUpdate   *bool       `yaml:"package_update,omitempty"`
	FinalMessage    string      `yaml:"final_message,omitempty"`
	WriteFiles      []WriteFile `yaml:"write_files,omitempty"`
	RunCommands     []string    `yaml:"runcmd,omitempty"`
	// BootCommands are commands you want to run early on in the boot process. These should only
	// be used for commands that are need early on and running them via RunCommands is too late.
	BootCommands       []string `yaml:"bootcmd,omitempty"`
	Mounts             []Mount  `yaml:"mounts,omitempty"`
	MountDefaultFields Mount    `yaml:"mount_default_fields,omitempty,flow"`
}

func (u *UserData) HasMountByName(deviceName string) bool {
	if len(u.Mounts) == 0 {
		return false
	}

	for _, mount := range u.Mounts {
		if mount[0] == deviceName {
			return true
		}
	}

	return false
}

func (u *UserData) HasMountByMountPoint(mountPoint string) bool {
	if len(u.Mounts) == 0 {
		return false
	}

	for _, mount := range u.Mounts {
		if mount[1] == mountPoint {
			return true
		}
	}

	return false
}

type Mount []string

type User struct {
	Name              string   `yaml:"name"`
	Sudo              string   `yaml:"sudo,omitempty"`
	Groups            string   `yaml:"groups,omitempty"`
	Home              string   `yaml:"home,omitempty"`
	Shell             string   `yaml:"shell,omitempty"`
	LockPasswd        *bool    `yaml:"lock_passwd,omitempty"`
	SSHAuthorizedKeys []string `yaml:"ssh_authorized_keys,omitempty"`
}

type WriteFile struct {
	Encoding    string `yaml:"encoding"`
	Content     string `yaml:"content"`
	Path        string `yaml:"path"`
	Permissions string `yaml:"permissions"`
}
