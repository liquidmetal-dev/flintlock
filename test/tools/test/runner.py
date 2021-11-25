from metal.welder import Welder


THINPOOL_NAME = "flintlock-thinpool"


class Test:
    def __init__(self, auth_token, config):
        self.testCfg = config['test']
        devCfg = config['device']
        self.welder = Welder(auth_token, config)
        self.skip_teardown = self.testCfg['skip_teardown']
        self.dev_id = devCfg['id']
        self.dev_ip = None

    def __enter__(self):
        return self

    def __exit__(self, *args, **kwargs):
        self.teardown()

    def setup(self):
        if self.dev_id is not None:
            self.fetch_infra()
        else:
            self.create_infra()

    def run_tests(self):
        cmd = ['./test/e2e/test.sh',
               '-level.flintlockd', self.testCfg['flintlock_log_level'],
               '-level.containerd', self.testCfg['containerd_log_level'],
               '-skip.setup.thinpool',
               '-thinpool', THINPOOL_NAME
               ]
        if self.testCfg['skip_delete']:
            cmd.append('-skip.teardown')
            cmd.append('-skip.delete')
        self.welder.run_ssh_command(cmd, "/root/work/flintlock", False)

    def teardown(self):
        if self.skip_teardown:
            return
        self.welder.delete_all()

    def create_infra(self):
        self.dev_ip, self.dev_id = self.welder.create_all()

    def fetch_infra(self):
        try:
            self.welder.set_key_dir()
            self.dev_ip = self.welder.get_device_ip(self.dev_id)
        except:
            raise

    def device_details(self):
        return self.dev_id, self.dev_ip
