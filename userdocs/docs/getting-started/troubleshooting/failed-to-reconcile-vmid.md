# Flintlockd fails to start

## Error: `failed to reconcile vmid`

Example error:

```
ERRO[0007] failed to reconcile vmid Hello/aa3b711d-4b60-4ba5-8069-0511c213308c:
  getting microvm spec for reconcile:
    getting vm spec from store:
      finding content in store:
        walking content store for aa3b711d-4b60-4ba5-8069-0511c213308c:
  context canceled  controller=microvm
```

There is a plan to create a VM, but something went wrong. The easiest way to
fix it to remove it from containerd:

```bash
vmid='aa3b711d-4b60-4ba5-8069-0511c213308c'
contentHash=$(\
  ctr-dev \
    --namespace=flintlock \
    content ls \
    | awk "/${vmid}/ {print \$1}" \
)
ctr-dev \
    --namespace=flintlock \
    content rm "${contentHash}"
```
