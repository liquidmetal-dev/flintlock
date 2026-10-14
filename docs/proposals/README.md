# Proposals

Design proposals for flintlock: requirements documents, research surveys and
design candidates that describe how a feature could be built. They sit
upstream of the decisions recorded in [`docs/adr/`](../adr/); once a proposal
is accepted, the decision it leads to should be captured as an ADR.

Each feature lives in its own directory named `YY-MM-DD-<slug>/`, where the
date is the day the directory was created.

## Index

### [26-09-25-snapshot-restore](26-09-25-snapshot-restore/)

MicroVM snapshot and restore. Tracking issue:
[#204](https://github.com/liquidmetal-dev/flintlock/issues/204).

Suggested reading order: requirements, then the options survey, then the
three design candidates, then the decision record.

| Document | Status |
| --- | --- |
| [MicroVM Snapshot & Restore Requirements](26-09-25-snapshot-restore/0204-snapshot-restore-requirements.md) | proposed |
| [Root Volume Options for Snapshot & Restore](26-09-25-snapshot-restore/0204-root-volume-options.md) | informative |
| [Option A: stay on devmapper (dm-thin)](26-09-25-snapshot-restore/0204-option-a-devmapper.md) | proposed design candidate |
| [Option B: containerd blockfile snapshotter on a reflink filesystem](26-09-25-snapshot-restore/0204-option-b-blockfile.md) | considered, may be revisited |
| [Option C: read-only deterministic base plus a per-VM writable disk](26-09-25-snapshot-restore/0204-option-c-ro-base-rw-disk.md) | proposed design candidate (selected) |
| [Root Volume Decision: implement Option C directly](26-09-25-snapshot-restore/0204-root-volume-decision.md) | proposed |

## Adding a proposal

1. Create `docs/proposals/YY-MM-DD-<slug>/` using today's date.
2. Put the documents for the feature in that directory. Keep cross-links
   between them relative so the set can be moved as a unit.
3. Add a section for the directory to the index above, with a row per
   document giving its title and status.
