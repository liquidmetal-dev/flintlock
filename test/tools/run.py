#!/usr/bin/env python3

from test import Test
from metal import Welder
import click
import os
import sys
import random
import string
from os.path import dirname, abspath

def generate_string():
    letters = string.ascii_letters
    return ( ''.join(random.choice(letters) for i in range(10)) )

def generated_project_name():
    return "flintlock_prj_"+generate_string()

def generated_key_name():
    return "flintlock_key_"+generate_string()

@click.group()
def cli():
    """
    General thing doer for flintlock
    """
    pass

@cli.command()
@click.option('-o', '--org-id', type=str, help='Equinix organisation id (required)')
@click.option('-p', '--project-name', type=str, help='Name of the project to create (default: randomly generated)', default=generated_project_name())
@click.option('-k', '--ssh-key-name', type=str, help='Name of the ssh key to create and attach to the device (default: randomly generated)', default=generated_key_name())
@click.option('-d', '--device-name', type=str, help='Name of the device to create (default: randomly generated)', default='T-800')
@click.option('-e', '--existing-device-id', type=str, help='Skip create and set the UUID of an existing device to run tests against', default=None)
@click.option('-s', '--skip-delete', is_flag=True, help='Skip cleanup of Equinix infrastructure for debugging after run', default=False)
def run_e2e(org_id, project_name, ssh_key_name, device_name, skip_delete, existing_device_id):
    token = os.environ.get("METAL_AUTH_TOKEN")
    if token is None:
        click.echo("must set METAL_AUTH_TOKEN")
        sys.exit()
    if org_id is None:
        click.echo("must set --org-id")
        sys.exit()

    if existing_device_id == None:
        click.echo(f"Running e2e tests. Will create project '{project_name}', ssh_key '{ssh_key_name}' and device '{device_name}'")
        click.echo("Note: this will create and bootstrap a new device in Equinix and may take some time")
    else:
        skip_delete = True
        tool_dir = dirname(abspath(__file__))
        if os.path.exists(tool_dir+"/private.key") != True:
            click.echo(f"`private.key` file must be saved at `{tool_dir}` when `--existing-device-id` flag set")
            sys.exit()
        click.echo("running e2e tests using device '{existing_device_id}`")

    runner = Test(token, org_id, project_name, ssh_key_name, device_name, skip_delete, existing_device_id)
    with runner:
        runner.setup()
        runner.run_tests()

    if skip_delete:
        dev_id, dev_ip = runner.device_details()
        click.echo(f"Device `{dev_id}` left alive for debugging. Use with `--existing-device-id` to re-run tests. SSH command `ssh -i hack/tools/private.key root@{dev_ip}`. Delete device with `delete-device` command.")

@cli.command()
@click.option('-o', '--org-id', type=str, help='Equinix organisation id (required)')
@click.option('-p', '--project-id', type=str, help='ID of the project to create the device in (required)')
@click.option('-k', '--ssh-key-name', type=str, help='Name of the ssh key to create and attach to the device (default: randomly generated)', default=generated_key_name())
@click.option('-d', '--device-name', type=str, help='Name of the device to create (default: randomly generated)', default='T-800')
@click.option('-u', '--userdata', type=str, help='String containing shell bootstrap userdata (default: standard flintlockd bootstrapping, see readme for details)')
def create_device(org_id, project_id, ssh_key_name, device_name, userdata):
    token = os.environ.get("METAL_AUTH_TOKEN")
    if token is None:
        click.echo("must set METAL_AUTH_TOKEN")
        sys.exit()
    if org_id is None:
        click.echo("must set --org-id")
        sys.exit()
    if project_id is None:
        click.echo("must set --project-id")
        sys.exit()

    click.echo(f"Creating device {device_name}")

    welder = Welder(token, org_id)
    ip = welder.create_all(project_id, device_name, ssh_key_name, userdata)

    click.echo(f"Device {device_name} created. SSH command `ssh -i hack/tools/private.key root@{ip}`. Run tests with `run-e2e`. Delete with `delete-device`.")

@cli.command()
@click.option('-o', '--org-id', type=str, help='Equinix organisation id (required)')
@click.option('-d', '--device-id', type=str, help='Name of the device to delete (required)')
def delete_device(org_id, device_id):
    token = os.environ.get("METAL_AUTH_TOKEN")
    if token is None:
        click.echo("must set METAL_AUTH_TOKEN")
        sys.exit()
    if org_id is None:
        click.echo("must set --org-id")
        sys.exit()
    if device_id is None:
        click.echo("must set --device-id")
        sys.exit()

    click.echo(f"Deleting device {device_id}")

    welder = Welder(token, org_id)
    welder.delete_device(device_id)

if __name__ == "__main__":
    cli()
