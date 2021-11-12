import yamale
from deepmerge import always_merger
import random
import string
from os.path import dirname, abspath


class Config:
    def __init__(self):
        self.dir = dirname(abspath(__file__))
        self.base = dirname(dirname(dirname(abspath(__file__))))
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
            'device': self.default_device_config(),
            'test': self.default_test_config()
        }
        self.params.update(data)

    def set_common(self, org_id=None, key_name=None, dev_name=None):
        if org_id is not None:
            self.params['org_id'] = org_id
        if key_name is not None:
            self.params['device']['ssh_key_name'] = key_name
        if dev_name is not None:
            self.params['device']['name'] = dev_name

    def set_run_flag_config(self, org_id=None, project_name=None, key_name=None, dev_name=None, skip_teardown=None):
        self.set_common(org_id, key_name, dev_name)

        if project_name is not None:
            self.params['project_name'] = project_name
        if skip_teardown is not None:
            self.params['test']['skip_teardown'] = skip_teardown

    def set_create_flag_config(self, org_id=None, project_id=None, key_name=None, dev_name=None):
        self.set_common(org_id, key_name, dev_name)

        if project_id is not None:
            self.params['project_id'] = project_id

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

        if self.params['device']['id'] is not None:
            self.params['test']['skip_teardown'] = True
            self.params['device']['name'] = None

        if self.params['test']['skip_delete'] is True:
            self.params['test']['skip_teardown'] = True

    def validate_create(self):
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

    def default_test_config(self):
        return {
            'skip_delete': False,
            'skip_teardown': False,
            'skip_dmsetup': False,
            'containerd_log_level': 'debug',
            'flintlock_log_level': '2',
        }

    def default_device_config(self):
        return {
            'name': self.default_device_name(),
            'id': None,
            'ssh_key_name': self.generated_key_name(),
            'userdata': self.default_user_data(),
            'plan': 'c1.small.x86',
            'operating_system': 'ubuntu_18_04',
            'metro': 'sv',
            'facility': 'ewr1',
            'billing_cycle': 'hourly'
        }

    def default_user_data(self):
        files = ["hack/scripts/bootstrap.sh", "test/tools/userdata.sh"]
        userdata = ""
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
