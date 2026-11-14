import packet
from packet import ResponseError
import time
import spur
import logging
import string
import random
import os
import sys
from .error import CapacityError
import shutil
from Crypto.PublicKey import RSA
from pathlib import Path
from os.path import dirname, abspath


class Welder():
    def __init__(self, auth_token, config, level=logging.INFO):
        self.config = config
        self.org_id = config['org_id']
        self.packet_manager = packet.Manager(auth_token=auth_token)
        self.logger = self.set_logger(level)
        self.ip = None
        self.project = None
        self.key = None
        self.key_dir = ""
        self.device = None

    def set_logger(self, level):
        logger = logging.getLogger('welder')
        logger.setLevel(level)
        c_handler = logging.StreamHandler()
        c_handler.setLevel(level)
        c_format = logging.Formatter(
            '%(asctime)s %(levelname)s %(name)s - %(message)s',
            "%Y-%m-%d.%H:%M:%S")
        c_handler.setFormatter(c_format)
        logger.addHandler(c_handler)

        return logger

    def create_all(self):
        facility = None
        try:
            facility = self.check_capacity()
            self.logger.info(f"using facility {facility}")
        except CapacityError as e:
            self.logger.error(f"{e}")
        project = self.create_new_project_if_not_exist()
        self.project = project
        key = self.create_new_key_if_not_exist(project.id)
        self.key = key
        device = self.create_device(project.id, key.id, facility)
        self.device = device
        self.dev_id = device.id
        self.wait_until_device_ready(device)

        return self.ip, self.device.id

    def check_capacity(self):
        for facility in self.config['device']['facility']:
            server = [(facility, self.config['device']['plan'], 1)]
            if self.packet_manager.validate_capacity(server):
                return facility
        raise CapacityError(
            'none of the given facilities have capacity for device')

    def create_new_project_if_not_exist(self):
        project = None
        try:
            project = self.packet_manager.get_project(
                self.config['project_id'])
            self.logger.info(
                f"using found project {self.config['project_id']}")
            return project
        except Exception:
            self.logger.info(
                f"could not find project {self.config['project_id']}")
            pass

        project = self.packet_manager.create_organization_project(
            org_id=self.org_id,
            name=self.config['project_name']
        )
        self.logger.info(f"created project {project.name}")

        return project

    def create_new_key_if_not_exist(self, project_id):
        self.set_key_dir()
        self.mk_key_dir()

        name = self.config['device']['ssh']['name']
        if self.config['device']['ssh']['create_new'] is False:
            self.logger.info(f"adding key {name} to device")
            return name

        key = RSA.generate(2048)
        with open(self.private_key(), 'wb') as priv_file:
            os.chmod(self.private_key(), 0o600)
            priv_file.write(key.exportKey('PEM'))

        pubkey = key.publickey().exportKey('OpenSSH')
        with open(self.public_key(), 'wb') as pub_file:
            pub_file.write(pubkey)

        key = self.packet_manager.create_project_ssh_key(
            project_id, name, pubkey.decode("utf-8"))
        self.logger.info(f"created key {key.label}")

        return key

    def create_device(self, project_id, key_id, facility):
        cfg = self.config['device']
        device = self.packet_manager.create_device(project_id=project_id,
                                                   hostname=cfg['name'],
                                                   plan=cfg['plan'], facility=facility,
                                                   operating_system=cfg['operating_system'],
                                                   billing_cycle=cfg['billing_cycle'],
                                                   project_ssh_keys=[key_id],
                                                   userdata=cfg['userdata'])
        self.logger.info(f"created device {device.hostname}")

        return device

    # TODO timeouts
    def wait_until_device_ready(self, device):
        while True:
            d = self.packet_manager.get_device(device_id=device.id)
            self.logger.info(f"checking state of device {d.hostname} ...")
            if d.state == "active":
                self.logger.info(f"{d.hostname} running")
                break
            self.logger.info(
                f"state '{d.state}' != 'active', sleeping for 10s")
            time.sleep(10)

        ips = device.ips()
        self.ip = ips[0].address

        while True:
            shell = self.new_shell(self.ip)
            self.logger.info("checking userdata has completed...")
            with shell:
                result = shell.run(
                    ["ls", "/flintlock_ready"], allow_error=True)
            if result.return_code == 0:
                self.logger.info("userdata ran successfully")
                break
            self.logger.info("userdata still running, sleeping for 10s")
            time.sleep(10)

        self.logger.info(f"device {device.hostname} (id: {device.id}) ready")

    def run_ssh_command(self, cmd, cwd, allow_error=True):
        shell = self.new_shell(self.ip)
        with shell:
            result = shell.run(
                command=cmd,
                cwd=cwd,
                stdout=sys.stdout.buffer,
                stderr=sys.stderr.buffer,
                allow_error=allow_error)
        if result.return_code != 0:
            raise result.to_error()
        self.logger.info("command exited with code %d", result.return_code)

    def delete_all(self):
        if self.device is not None:
            self.device.delete()
            self.logger.info(f"deleted device {self.device.hostname}")

        if self.key is not None and self.config['device']['ssh']['create_new']:
            self.key.delete()
            shutil.rmtree(self.key_dir)
            self.logger.info(f"deleted key {self.key.label}")

        if self.project is not None and self.config['project_id'] is None:
            self.project.delete()
            self.logger.info(f"deleted project {self.project.name}")

    def new_shell(self, ip):
        shell = spur.SshShell(
            hostname=ip,
            username="root",
            load_system_host_keys=False,
            missing_host_key=spur.ssh.MissingHostKey.accept,
            private_key_file=self.private_key()
        )

        return shell

    def get_device(self, device_id):
        try:
            return self.packet_manager.get_device(device_id=device_id)
        except:
            raise ValueError(f"device {device_id} not found")

    def get_device_ip(self, device_id):
        d = self.get_device(device_id)
        ips = d.ips()
        self.ip = ips[0].address

        return self.ip

    def delete_device(self, device_id):
        device = self.get_device(device_id)
        device.delete()
        self.logger.info(f"deleted device {device.hostname}")

    def set_key_dir(self):
        ssh_cfg = self.config['device']['ssh']
        key_dir = f"/tmp/{ssh_cfg['name']}/keys"
        try:
            path = ssh_cfg['path']
            if path is not None:
                key_dir = f'{path}/keys'
                self.logger.info(
                    f'using key path: {key_dir}')
        except ValueError:
            pass

        self.key_dir = key_dir

    def mk_key_dir(self):
        if self.config['device']['ssh']['create_new'] is False:
            return

        Path(self.key_dir).mkdir(parents=True, exist_ok=True)
        self.logger.info(
            f'created dir for keys: {self.key_dir}')

    def private_key(self):
        return self.key_dir + '/private.key'

    def public_key(self):
        return self.key_dir + '/public.key'
