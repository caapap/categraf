# Ascend Common SDK - Vendor Information

## Source

This directory contains the `ascend-common` SDK from Huawei Ascend project.

**Original Repository:** https://gitcode.com/Ascend/mind-cluster/tree/master/component/ascend-common

**License:** Apache License 2.0

## Version Information

- **Cloned Date:** 2025-11-15
- **Repository:** mind-cluster
- **Branch:** master
- **Component:** ascend-common

## Purpose

The `ascend-common` SDK provides the core APIs and utilities for interacting with Huawei Ascend NPU devices:

- **api/**: Core API definitions for NPU operations
- **devmanager/**: Device manager for NPU discovery and management
- **common-utils/**: Common utilities (logging, caching, etc.)

## Integration

This SDK is used by the Categraf NPU input plugin (`inputs/npu/`) to collect metrics from Ascend NPU devices.

## Updates

To update this SDK to a newer version:

```bash
# Clone the latest version
cd /tmp
git clone https://gitcode.com/Ascend/mind-cluster.git --depth 1

# Copy to categraf
cp -r /tmp/mind-cluster/component/ascend-common /path/to/categraf/pkg/

# Clean up
rm -rf /tmp/mind-cluster

# Update go.mod if needed
cd /path/to/categraf
go mod tidy
```

## Dependencies

The ascend-common SDK has its own dependencies defined in `go.mod` and `go.sum`. These are managed automatically by Go modules.

## Notes

- This is a vendored copy to ensure Categraf is self-contained
- The SDK requires Ascend driver and toolkit to be installed on the target system
- CGO may be required depending on the SDK implementation

