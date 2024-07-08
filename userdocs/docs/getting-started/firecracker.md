---
title: Firecracker
---

We have to use a custom built firecracker from the `feature/macvtap` branch
([see][discussion-107]).

We compile and release our own binaries from this feature branch [here][fc].
Our fork is regularly updated from the upstream.

Consult the [compatibility table][compat] to ensure you install the correct version
for your `flintlock`.

Firecracker does not run as a service; flintlock will execute the binary and each MicroVM
will run as a `firecracker` process.

:::note
If you are feeling weird, you can build this yourself, but we don't recommend it:

```bash
git clone https://github.com/liquidmetal-dev/firecracker.git
git fetch origin feature/macvtap
git checkout -b feature/macvtap origin/feature/macvtap

# This will build it in a docker container, no rust installation required.
tools/devtool build

# Any directories on $PATH.
TARGET=~/local/bin
toolbox=$(uname -m)-unknown-linux-musl

cp build/cargo_target/${toolbox}/debug/{firecracker,jailer} ${TARGET}
```

:::

[discussion-107]: https://github.com/liquidmetal-dev/flintlock/discussions/107
[fc]: https://github.com/liquidmetal-dev/firecracker/releases
[compat]: https://github.com/liquidmetal-dev/flintlock#compatibility
