package models

/*
{
  "boot-source": {
    "kernel_image_path": "vmlinux.bin",
    "boot_args": "console=ttyS0 reboot=k panic=1 pci=off",
    "initrd_path": ""
  },
  "drives": [
    {
      "drive_id": "rootfs",
      "path_on_host": "bionic.rootfs.ext4",
      "is_root_device": true,
      "partuuid": "",
      "is_read_only": false,
      "cache_type": "Unsafe",
      "rate_limiter": ""
    }
  ],
  "machine-config": {
    "vcpu_count": 2,
    "mem_size_mib": 1024,
    "ht_enabled": false,
    "track_dirty_pages": false
  },
  "balloon": {
		"amount_mib": 0,
		"deflate_on_oom": false,
		"stats_polling_interval_s": 1
	},
	"network-interfaces": [
		{
			"iface_id": "eth0",
			"host_dev_name": "",
			"allow_mmds_requests": true
		}
	],
  "vsock": {
		"guest_cid": 3,
		"uds_path": "./v.sock"
	},
  "logger": {
		"log_path": "logs.fifo",
		"level": "Warning",
		"show_level": false,
		"show_log_origin": false
	},
  "metrics": {
		"metrics_path": "metrics.fifo"
	},
  "mmds-config": {}
}
*/
