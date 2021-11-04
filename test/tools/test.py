from metal import Welder

class Test:
    def __init__(self, auth_token, org_id, prj_name, key_name, dev_name, skip_delete, dev_id=None):
        self.welder = Welder(auth_token, org_id)
        self.prj_name = prj_name
        self.key_name = key_name
        self.dev_name = dev_name
        self.skip_delete = skip_delete
        self.dev_id = dev_id
        self.dev_ip = None
        self.project = None
        self.key = None
        self.device = None

    def __enter__(self):
        return self

    def __exit__(self, *args, **kwargs):
        if self.skip_delete == False:
            self.teardown()

    def setup(self):
        if self.dev_id != None:
            self.fetch_infra()
        else:
            self.create_infra()

    def run_tests(self):
        cmd = ['make', 'test-e2e']
        self.welder.run_ssh_command(cmd, "/root/work/flintlock")

    def teardown(self):
        self.welder.delete_all(self.project, self.device, self.key)

    def create_infra(self):
        self.dev_ip = self.welder.create_all(self.prj_name, self.dev_name, self.key_name)

    def fetch_infra(self):
        try:
            ip = self.welder.get_device_ip(self.dev_id)
        except:
            raise
        self.ip = ip

    def device_details(self):
        return self.dev_id, self.dev_ip
