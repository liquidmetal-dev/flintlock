# Releasing Flintlock

> These instructions will change when we start supporting previous versions whilst using main for future versions.

## Determine release version

The projects follows [semantic versioning](https://semver.org/#semantic-versioning-200) and so the release version must adhere to this specification. Depending on the changes in the release you will need to decide the next appropriate version number.

Its advised that you pull the tags and view the latest release (i.e. tag):

```bash
git pull --tags

git describe --tags --abbrev=0
```

## Create tag

* Checkout upstream main
* Create a tag with the version number:

```bash
RELEASE_VERSION=v0.1.0-alpha.1
git tag -s "${RELEASE_VERSION}" -m "${RELEASE_VERSION}"
```

* Push the tag (to upstream if working from a fork)

``` bash
git push origin "${RELEASE_VERSION}"
```

* Check the [release](https://github.com/weaveworks-liquidmetal/flintlock/actions/workflows/release.yml) GitHub Actions workflow completes successfully.
  This may take a few minutes as it runs the e2es as part of the process.

## Edit & Publish GitHub Release

* Got to the draft release in GitHub.
* Make any edits to generated release notes
  * If there are any breaking changes then manually add a note at the beginning of the release notes informing the user what they need to be aware of/do.
  * Sometimes you may want to combine changes into 1 line
* If this is a pre-release tick `This is a pre-release`
* Publish the draft release and when asked say yes to creating a discussion.

## Commit a new `buf` tag

We have gRPC API docs hosted on [buf.build](https://buf.build/weaveworks-liquidmetal/flintlock).
If the API has changed, you'll need to update these.

Log in creds can be found in the shared Team Quicksilver 1Pass vault.

Generate a new token.

Log in locally:

```
buf registry login
# username is weaveworks-liquidmetal
# key is the token you generated
```

Push a new tag:

```
buf push --tag "${RELEASE_VERSION}"
```

If you get the message `The latest commit has the same content; not creating a new commit.`
then it means there hasn't been changes to the API so you didn't need to do this.

## Announce release

When the release is available announce it in the #liquid-metal slack channel.
