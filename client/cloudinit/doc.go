// Package cloudinit contains types related to [cloudinit](https://cloudinit.readthedocs.io/en/latest/index.html)
// to be used by flintlock and its clients.
//
// In the struct definitions we have tried to remove usage of pointers and instead rely on the behavior of json
// marshalling and `omitempty`. Its slightly tricky with boolean values so for these we use *bool vs bool. The
// reason is that the default value for some cloudinit values is true which isn't the same as the default for bool.
package cloudinit
