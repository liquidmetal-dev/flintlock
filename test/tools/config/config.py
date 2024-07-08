import yamale
from deepmerge import always_merger
import random
import string
import os
from os.path import dirname, abspath


class Config:
    def __init__(self):
        self.dir = dirname(abspath(__file__))
        self.base = dirname(dirname(dirname(dirname(abspath(__file__)))))
        self.schema = self.dir + '/schema.yaml'
        self.params = {}
        self.set_default_config()

    def __getitem__(self, attr):
        return self.params[attr]

    def __setitem__(self, key, value):
        self.params[key] = value

    def set_default_config(self):
        data = {
            'org_id': None,
            'project_id': None,
            'project_name': self.generated_project_name(),
            'repo': self.default_repo_config(),
            'device': self.initial_device_config(),
            'test': self.initial_test_config()
        }
        self.params.update(data)

    def set_run_flag_config(self, org_id=None, project_name=None, dev_name=None, skip_teardown=None):
        self.set_common_flags(org_id, dev_name)

        if project_name is not None:
            self.params['project_name'] = project_name
        if skip_teardown is not None:
            self.params['test']['skip_teardown'] = skip_teardown

    def set_create_flag_config(self, org_id=None, project_id=None, dev_name=None):
        self.set_common_flags(org_id, dev_name)

        if project_id is not None:
            self.params['project_id'] = project_id

    def set_common_flags(self, org_id=None, dev_name=None):
        if org_id is not None:
            self.params['org_id'] = org_id
        if dev_name is not None:
            self.params['device']['name'] = dev_name

    def load_config_from_file(self, config_file):
        try:
            schema = yamale.make_schema(self.schema)
            data = yamale.make_data(config_file)
            yamale.validate(schema, data)
        except ValueError as e:
            raise e
        self.params = always_merger.merge(self.params, data[0][0])

    def validate_run(self):
        self.validate_common()
        ssh_cfg = self.params['device']['ssh']
        if self.params['device']['id'] is not None:
            self.validate_ssh_path('device.id')
        elif ssh_cfg['create_new'] is False:
            self.validate_ssh_name('device.id')
            self.validate_ssh_path('device.ssh.create_new: false')

        self.configure_test()
        self.configure_device()

    def validate_create(self):
        self.validate_common()
        self.configure_device()

        if self.params['project_id'] is None:
            raise ValueError("must set project_id")

        try:
            if self.params['device']['id'] is not None:
                raise ValueError(
                    "Error validating config: device.id should not be set")
        except KeyError:
            pass

        if self.params['device']['ssh']['create_new'] is False:
            self.validate_ssh_name('device.ssh.create_new: false')
            self.validate_ssh_path('device.ssh.create_new: false')

    def validate_common(self):
        if self.params['org_id'] is None:
            raise ValueError("must set org_id")

        if len(self.params['device']['facility']) < 1:
            raise ValueError("must set at least one facility")

    def validate_ssh_path(self, field):
        path = self.params['device']['ssh']['path']
        if path is None:
            raise ValueError(
                    f"must set device.ssh.path when setting {field}")
        if os.path.exists(f"{path}/keys/private.key") is False:
            raise ValueError(
                f"device.ssh.path {path} must contain `keys/private.key`")

    def validate_ssh_name(self, field):
        if self.params['device']['ssh']['name'] is None:
            raise ValueError(
                    f"must set device.ssh.name when setting {field}")


    def configure_test(self):
        if self.params['device']['id'] is not None:
            self.params['test']['skip_teardown'] = True
            self.params['device']['name'] = None

        if self.params['test']['skip_delete'] is True:
            self.params['test']['skip_teardown'] = True

    def configure_device(self):
        if self.params['device']['userdata'] is None:
            self.params['device']['userdata'] = self.default_user_data()
        if self.params['device']['ssh']['name'] is None:
            self.params['device']['ssh']['name'] = self.generated_key_name()
        if self.params['device']['id'] is not None:
            self.params['device']['ssh']['create_new'] = False

    def default_repo_config(self):
        return {
            'username': 'liquidmetal-dev',
            'branch': 'main'
        }

    def initial_test_config(self):
        return {
            'skip_delete': False,
            'skip_teardown': False,
            'containerd_log_level': 'debug',
            'flintlock_log_level': '2',
        }

    def initial_device_config(self):
        return {
            'skip_dmsetup': False,
            'name': self.default_device_name(),
            'id': None,
            'ssh': {
                'create_new': True,
                'name': None,
                'path': None,
            },
            'userdata': None,
            'plan': 'c3.small.x86',
            'operating_system': 'ubuntu_20_04',
            'facility': ['am6', 'ams1', 'fr2', 'fra2'],
            'billing_cycle': 'hourly'
        }

    def default_user_data(self):
        userdata = ""
        files = ["test/tools/config/userdata.sh"]

        if self.params['device']['skip_dmsetup'] is True:
            userdata += ("#!/bin/bash\n"
                         "export SKIP_DIRECT_LVM=true\n"
                         )

        userdata += ("#!/bin/bash\n"
                     f"export FL_USER={self.params['repo']['username']}\n"
                     f"export FL_BRANCH={self.params['repo']['branch']}\n"
                     )

        for file in files:
            with open(self.base + "/" + file) as f:
                userdata += f.read()
                userdata += "\n"

        return userdata

    def generate_string(self):
        letters = string.ascii_letters
        return (''.join(random.choice(letters) for _ in range(10)))

    def generated_project_name(self):
        return "flintlock_prj_" + self.generate_string()

    def generated_key_name(self):
        return "flintlock_key_" + self.generate_string()

    def default_device_name(self):
        return 'T-800'
