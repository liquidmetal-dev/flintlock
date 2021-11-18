import yamale
from deepmerge import always_merger
import random
import string
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
            'device': self.initial_device_config(),
            'test': self.initial_test_config()
        }
        self.params.update(data)

    def set_run_flag_config(self, org_id=None, project_name=None, key_name=None, dev_name=None, skip_teardown=None):
        self.set_common_flags(org_id, key_name, dev_name)

        if project_name is not None:
            self.params['project_name'] = project_name
        if skip_teardown is not None:
            self.params['test']['skip_teardown'] = skip_teardown

    def set_create_flag_config(self, org_id=None, project_id=None, key_name=None, dev_name=None):
        self.set_common_flags(org_id, key_name, dev_name)

        if project_id is not None:
            self.params['project_id'] = project_id

    def set_common_flags(self, org_id=None, key_name=None, dev_name=None):
        if org_id is not None:
            self.params['org_id'] = org_id
        if key_name is not None:
            self.params['device']['ssh_key_name'] = key_name
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
        if self.params['org_id'] is None:
            raise ValueError("must set org_id")

        self.configure_test()
        self.configure_device()

    def validate_create(self):
        self.configure_device()

        if self.params['org_id'] is None:
            raise ValueError("must set org_id")

        if self.params['project_id'] is None:
            raise ValueError("must set project_id")

        try:
            if self.params['device']['id'] is not None:
                raise ValueError(
                    "Error validating config: device.id should not be set")
        except KeyError:
            pass

    def configure_test(self):
        if self.params['device']['id'] is not None:
            self.params['test']['skip_teardown'] = True
            self.params['device']['name'] = None

        if self.params['test']['skip_delete'] is True:
            self.params['test']['skip_teardown'] = True

    def configure_device(self):
        if self.params['device']['userdata'] is None:
            self.params['device']['userdata'] = self.default_user_data()

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
            'ssh_key_name': self.generated_key_name(),
            'userdata': None,
            'plan': 'c3.small.x86',
            'operating_system': 'ubuntu_18_04',
            'metro': 'sv',
            'facility': 'am6',
            'billing_cycle': 'hourly'
        }

    def default_user_data(self):
        userdata = ""
        files = ["hack/scripts/bootstrap.sh", "test/tools/config/userdata.sh"]

        if self.params['device']['skip_dmsetup'] is False:
            files.insert(1, "hack/scripts/direct_lvm.sh")
            userdata += "#!/bin/bash\n export THINPOOL_DISK_NAME=sdb\n"

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
