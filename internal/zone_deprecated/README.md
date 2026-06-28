# Zone-Based Replication (Deprecated)

This directory contains the deprecated zone-based replication model that was used in AmorphDB prior to Step 14.

## What Was Here

The zone-based model used consistent hashing to distribute data across zones, where each zone had:
- An authority node (responsible for writes)
- One or more replica nodes (for redundancy)
- Zone splitting logic for load balancing

## Replacement

This model was replaced in Step 14 with subscription-based replication, where:
- Each path has an explicit authority node
- Nodes subscribe to paths they want to replicate
- Write routing is explicit rather than hash-based
- Authority assignment uses longest-prefix matching

## Files

- `hashring.go` / `hashring_test.go` - Consistent hashing implementation
- `split.go` / `split_test.go` - Zone splitting logic for load balancing  
- `step12_2_test.go` - Zone-based integration tests

## Do Not Delete

These files are preserved for reference during any debugging or migration issues.
They should not be imported by any active code.