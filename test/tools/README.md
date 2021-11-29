# run.py

A handy tool to run tests in and interact with Equinix devices.

Install dependencies:

```bash
pip3 install -r test/tools/requirements.txt
```

Run the tool:

```bash
./test/tools/run.py
Usage: run.py [OPTIONS] COMMAND [ARGS]...

  General thing doer for flintlock

Options:
  --help  Show this message and exit.

Commands:
  create-device
  delete-device
  run-e2e
```

Run `--help` on all those subcommands to see the run options.

For all commands you will need your Organisation ID and to set your API token
in the environment as `METAL_AUTH_TOKEN`.

## Running the e2es

```bash
export METAL_AUTH_TOKEN=<your token>
./test/tools/run.py run-e2e --org-id <your org id>
```

This will:
- Create a new project in your org
- Create a new ssh key, saving the files locally for debugging
- Create a new device
- Bootstrap the device with userdata
- Wait for the device to be running
- Wait for the userdata to complete bootstrapping
- Run the e2e tests, streaming the output
- Delete the device, key, and project

To keep the device around for debugging, add `--skip-delete`.

## Creating a device

```bash
export METAL_AUTH_TOKEN=<your token>
./test/tools/run.py create-device --org-id <your org id> --project-id <existing project-id>
```

This will create a device in an existing project bootstrapped with the default userdata.
You can use `--userdata` to override this.

Nothing will be cleaned up afterwards.

## Deleting a device

```bash
export METAL_AUTH_TOKEN=<your token>
./test/tools/run.py delete-device --org-id <your org id> --device-id <existing device-id>
```

This will delete the given device. The project will not be deleted.

## Advanced config

The `run-e2e` and `create-device` commands both support receiving params via config
file.

```bash
export METAL_AUTH_TOKEN=<your token>
./test/tools/run.py run-e2e --config-file <path to yaml>
```

With a config file you can do more, for example:

##### `run-e2e`
- Run tests in an existing device, without spinning up fresh infra (`device.id`)
- Configure the device which is created
- Configure test parameters (`test.flintlockd_log_level`, `test.containerd_log_level`, etc)

##### `create-device`
- Create a device in an existing project
- Configure the device which is created

Not all device configuration is exposed yet, but is it fairly trivial to add more when
required.

To see all available configuration, see the [example config](tests/tools/example-config.yaml).
