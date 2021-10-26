# -*- mode: ruby -*-
# vi: set ft=ruby :

# Copyright The flintlock Authors

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at

#     http://www.apache.org/licenses/LICENSE-2.0

# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

Vagrant.configure("2") do |config|
  config.vm.box = "generic/ubuntu2004"

  config.ssh.forward_agent = true
  config.vm.synced_folder "./", "/home/vagrant/flintlock"

  config.vm.network "forwarded_port", guest: 9090, host: 9090

  cpus = 2
  memory = 4096
  config.vm.provider :virtualbox do |v|
    # Enable nested virtualisation in VBox
    v.customize ["modifyvm", :id, "--nested-hw-virt", "on"]

    v.cpus = cpus
    v.memory = memory
  end
  config.vm.provider :libvirt do |v, override|
    # If you want to use a different storage pool.
    # v.storage_pool_name = "vagrant"
    v.cpus = cpus
    v.memory = memory
    override.vm.synced_folder "./", "/home/vagrant/flintlock", type: "nfs"
  end

  config.vm.provision "upgrade-packages", type: "shell", run: "once" do |sh|
    sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eux -o pipefail
      apt update && apt upgrade -y
    SHELL
  end

  config.vm.provision "install-basic-packages", type: "shell", run: "once" do |sh|
    sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eux -o pipefail
      apt install -y \
        make \
        git \
        gcc \
        curl \
        unzip \
        containerd
    SHELL
  end

  config.vm.provision "install-golang", type: "shell", run: "once" do |sh|
    sh.env = {
      'GO_VERSION': ENV['GO_VERSION'] || "1.17.2",
    }
    sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eux -o pipefail
      curl -fsSL "https://dl.google.com/go/go${GO_VERSION}.linux-amd64.tar.gz" | tar Cxz /usr/local
      cat >> /etc/environment <<EOF
PATH=/usr/local/go/bin:$PATH
EOF
      source /etc/environment
      cat >> /etc/profile.d/sh.local <<EOF
GOPATH=\\$HOME/go
PATH=\\$GOPATH/bin:\\$PATH
export GOPATH PATH
EOF
    source /etc/profile.d/sh.local
    SHELL
  end

  config.vm.provision "configure-thinpool", type: "shell",
    run: "once", path: "./hack/scripts/devpool.sh"

  config.vm.provision "configure-containerd", type: "shell", run: "once" do |sh|
    sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eux -o pipefail

      # ensure directories exist
      mkdir -p /etc/containerd
      mkdir -p /var/lib/containerd-dev/snapshotter/devmapper
      mkdir -p /run/containerd-dev/

      cp /home/vagrant/flintlock/hack/scripts/example-config.toml /etc/containerd/config.toml

      systemctl restart containerd
    SHELL
  end

  config.vm.provision "install-kvm", type: "shell", run: "once" do |sh|
    sh.inline = <<~SHELL
      #!/usr/bin/env bash
      set -eux -o pipefail
      apt install -y qemu-kvm libvirt-daemon-system libvirt-clients bridge-utils
      adduser 'vagrant' libvirt
      adduser 'vagrant' kvm
      setfacl -m u:${USER}:rw /dev/kvm
    SHELL
  end

  config.vm.provision "install-firecracker", type: "shell", run: "once" do |sh|
    sh.inline = <<~SHELL
      curl -fsSL "https://github.com/weaveworks/reignite/files/7278467/firecracker_macvtap.zip" -o /tmp/firecracker-macvtap.zip
      unzip -u /tmp/firecracker-macvtap.zip -d /usr/local/bin
    SHELL
  end

end
