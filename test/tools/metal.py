import packet
import time
import spur
import logging
import os
import sys
from Crypto.PublicKey import RSA
from os.path import dirname, abspath

class Welder():
   def __init__(self, auth_token, org_id, level=logging.INFO):
      self.org_id = org_id
      self.base = dirname(dirname(dirname(abspath(__file__))))
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

   def create_all(self, prj_name_or_id, dev_name, key_name, userdata=None):
      project = self.create_new_project_if_not_exist(prj_name_or_id)
      self.project = project
      key = self.create_new_key(project.id, key_name)
      self.key = key
      if userdata == None:
         userdata = self.create_user_data()
      device = self.create_device(project.id, dev_name, key.id, userdata)
      self.device = device
      self.dev_id = device.id
      self.wait_until_device_ready(device)

      return self.ip

   def create_new_project_if_not_exist(self, name_or_id):
      project = None
      try:
         project = self.packet_manager.get_project(name_or_id)
         self.logger.info(f"using found project {name_or_id}")
         return project
      except Exception:
         pass

      project = self.packet_manager.create_organization_project(
          org_id=self.org_id,
          name=name_or_id
      )
      self.logger.info(f"created project {name_or_id}")

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

   def create_user_data(self):
      files = ["hack/scripts/bootstrap.sh", "hack/scripts/devpool.sh", "test/tools/userdata.sh"]
      userdata = ""
      for file in files:
         with open(self.base+"/"+file) as f:
             userdata += f.read()
             userdata += "\n"

      return userdata

   def create_device(self, project_id, name, key_id, userdata):
      device = self.packet_manager.create_device(project_id=project_id,
                                     hostname=name,
                                     plan='c1.small.x86', metro='sv',
                                     operating_system='ubuntu_18_04',
                                     facility="ewr1", billing_cycle='hourly',
                                     project_ssh_keys=[key_id],
                                     userdata=userdata)
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

   def run_ssh_command(self, cmd, cwd):
      shell = self.new_shell(self.ip)
      with shell:
         result = shell.run(command=cmd, cwd=cwd, stdout=sys.stdout.buffer, stderr=sys.stderr.buffer, allow_error=True)
      self.logger.info("command exited with code %d", result.return_code)

   def delete_all(self, project, device, key):
      if device != None:
         device.delete()
         self.logger.info(f"deleted device {device.hostname}")

      if key != None:
         key.delete()
         os.remove(self.private_key_path)
         os.remove(self.public_key_path)
         self.logger.info(f"deleted key {key.label}")

      if project != None:
         project.delete()
         self.logger.info(f"deleted project {project.name}")

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
