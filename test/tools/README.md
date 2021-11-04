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

To not creating anything new and run the tests in an existing device, set `--existing-device-id <id>`.

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
./test/tools/run.py delete-device --org-id <your org id> --device-id <existing project-id>
```

This will delete the given device. The project will not be deleted.
