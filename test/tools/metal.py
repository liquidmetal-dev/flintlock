import packet
import time
import spur
import logging
import os
import sys
from Crypto.PublicKey import RSA
from os.path import dirname, abspath

class Welder():
   def __init__(self, auth_token, config, level=logging.INFO):
      self.config = config
      self.org_id = config['org_id']
      self.packet_manager = packet.Manager(auth_token=auth_token)
      self.private_key_path = dirname(abspath(__file__))+"/private.key"
      self.public_key_path = dirname(abspath(__file__))+"/public.key"
      self.logger = self.set_logger(level)
      self.ip = None

   def set_logger(self, level):
      logger = logging.getLogger('welder')
      logger.setLevel(level)
      c_handler = logging.StreamHandler()
      c_handler.setLevel(level)
      c_format = logging.Formatter('%(asctime)s %(levelname)s %(name)s - %(message)s', "%Y-%m-%d.%H:%M:%S")
      c_handler.setFormatter(c_format)
      logger.addHandler(c_handler)

      return logger

   def create_all(self):
      project = self.create_new_project_if_not_exist()
      self.project = project
      key = self.create_new_key(project.id, self.config['device']['ssh_key_name'])
      self.key = key
      device = self.create_device(project.id, key.id)
      self.device = device
      self.dev_id = device.id
      self.wait_until_device_ready(device)

      return self.ip

   def create_new_project_if_not_exist(self):
      project = None
      try:
         project = self.packet_manager.get_project(self.config['project_id'])
         self.logger.info(f"using found project {self.config['project_id']}")
         return project
      except Exception:
         self.logger.info(f"could not find project {self.config['project_id']}")
         pass

      project = self.packet_manager.create_organization_project(
          org_id=self.org_id,
          name=self.config['project_name']
      )
      self.logger.info(f"created project {project.name}")

      return project

   def create_new_key(self, project_id, name):
      key = RSA.generate(2048)
      with open(self.private_key_path, 'wb') as priv_file:
          os.chmod(self.private_key_path, 0o600)
          priv_file.write(key.exportKey('PEM'))

      pubkey = key.publickey().exportKey('OpenSSH')
      with open(self.public_key_path, 'wb') as pub_file:
          pub_file.write(pubkey)

      key = self.packet_manager.create_project_ssh_key(project_id, name, pubkey.decode("utf-8"))
      self.logger.info(f"created key {key.label}")

      return key

   def create_device(self, project_id, key_id):
      cfg = self.config['device']
      device = self.packet_manager.create_device(project_id=project_id,
                                     hostname=cfg['name'],
                                     plan=cfg['plan'], metro=cfg['metro'],
                                     operating_system=cfg['operating_system'],
                                     facility=cfg['facility'], billing_cycle=cfg['billing_cycle'],
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
         self.logger.info(f"state '{d.state}' != 'active', sleeping for 10s")
         time.sleep(10)

      ips = device.ips()
      self.ip = ips[0].address

      while True:
         shell = self.new_shell(self.ip)
         self.logger.info("checking userdata has completed...")
         with shell:
            result = shell.run(["ls", "/flintlock_ready"], allow_error=True)
         if result.return_code == 0:
            self.logger.info("userdata ran successfully")
            break
         self.logger.info("userdata still running, sleeping for 10s")
         time.sleep(10)

      self.logger.info(f"device {device.hostname} (id: {device.id}) ready")

   def run_ssh_command(self, cmd, cwd, allow_error=True):
      shell = self.new_shell(self.ip)
      with shell:
         result = shell.run(command=cmd, cwd=cwd, stdout=sys.stdout.buffer, stderr=sys.stderr.buffer, allow_error=allow_error)
      if result.return_code != 0:
         raise result.to_error()
      self.logger.info("command exited with code %d", result.return_code)

   def delete_all(self):
      if self.device is not None:
         self.device.delete()
         self.logger.info(f"deleted device {self.device.hostname}")

      if self.key is not None:
         self.key.delete()
         os.remove(self.private_key_path)
         os.remove(self.public_key_path)
         self.logger.info(f"deleted key {self.key.label}")

      if self.project is not None:
         self.project.delete()
         self.logger.info(f"deleted project {self.project.name}")

   def new_shell(self, ip):
      shell = spur.SshShell(
         hostname=ip,
         username="root",
         load_system_host_keys=False,
         missing_host_key=spur.ssh.MissingHostKey.accept,
         private_key_file=self.private_key_path
      )

      return shell

   def get_device(self, device_id):
      return self.packet_manager.get_device(device_id=device_id)

   def get_device_ip(self, device_id):
      d = self.get_device(device_id)
      ips = d.ips()
      self.ip = ips[0].address

      return self.ip

   def delete_device(self, device_id):
      device = self.get_device(device_id)
      device.delete()
      self.logger.info(f"deleted device {device.hostname}")
