# Ascend Common SDK

This directory contains the Huawei Ascend Common SDK, which provides core APIs for interacting with Ascend NPU devices.

## Source

**Repository:** https://gitcode.com/Ascend/mind-cluster  
**Component:** ascend-common  
**License:** Apache License 2.0  
**Integrated Date:** 2025-11-15

## Directory Structure

```
ascend-common/
├── api/                # Core API definitions
│   ├── ascend_api.go
│   └── ...
├── devmanager/         # Device manager
│   ├── device_manager.go
│   ├── common/
│   └── ...
├── common-utils/       # Common utilities
│   ├── hwlog/         # Logging
│   ├── cache/         # Caching
│   ├── limiter/       # Rate limiting
│   └── ...
├── go.mod             # Go module definition
├── go.sum             # Go module checksums
└── VENDOR_INFO.md     # Vendor information
```

## Purpose

This SDK is used by the Categraf NPU input plugin (`inputs/npu/`) to:

1. **Discover NPU devices** - Enumerate all available NPU cards and chips
2. **Collect metrics** - Gather temperature, power, utilization, memory usage, etc.
3. **Manage devices** - Initialize, configure, and monitor NPU devices
4. **Handle errors** - Retrieve error codes and health status

## Integration

The SDK is integrated into Categraf via Go modules:

```go
// go.mod
require (
    ascend-common v0.0.0
)

replace (
    ascend-common => ./pkg/ascend-common
)
```

## Usage in Categraf

The NPU plugin uses the following key components:

### Device Manager

```go
import "ascend-common/devmanager"

// Initialize device manager
dmgr, err := devmanager.AutoInit("", 0)

// Get card list
cardNum, cards, err := dmgr.GetCardList()

// Get device info
logicID, err := dmgr.GetDeviceLogicID(cardID, deviceID)
```

### Metrics Collection

```go
import "ascend-common/devmanager/common"

// Get temperature
temp, err := dmgr.GetDeviceTemperature(logicID)

// Get power
power, err := dmgr.GetDevicePowerInfo(logicID)

// Get utilization
util, err := dmgr.GetDeviceUtilizationRate(logicID, common.AICore)

// Get memory info
memInfo, err := dmgr.GetDeviceMemoryInfo(logicID)
```

## Requirements

### Runtime Requirements

1. **Ascend Driver** - NPU driver must be installed
   ```bash
   cat /proc/driver/ascend/version
   ```

2. **Device Access** - Process must have permission to access `/dev/davinci*`
   ```bash
   ls -l /dev/davinci*
   sudo usermod -a -G HwHiAiUser $USER
   ```

3. **Ascend Toolkit** (Optional) - For development and debugging
   ```bash
   # Typical installation path
   /usr/local/Ascend/ascend-toolkit/latest/
   ```

### Build Requirements

- Go 1.18 or later
- CGO may be required (depends on SDK implementation)
- C compiler (gcc) if CGO is needed

## Updating

To update to a newer version of ascend-common:

```bash
# 1. Clone the latest version
cd /tmp
git clone https://gitcode.com/Ascend/mind-cluster.git --depth 1

# 2. Backup current version
cd /path/to/categraf
mv pkg/ascend-common pkg/ascend-common.bak

# 3. Copy new version
cp -r /tmp/mind-cluster/component/ascend-common pkg/

# 4. Update dependencies
go mod tidy

# 5. Test
go build
./categraf --test --inputs npu

# 6. Clean up
rm -rf /tmp/mind-cluster
rm -rf pkg/ascend-common.bak  # if test successful
```

## Troubleshooting

### Build Errors

**Error: CGO is required**
```bash
export CGO_ENABLED=1
export CC=gcc
go build
```

**Error: Cannot find C libraries**
```bash
export CGO_CFLAGS="-I/usr/local/Ascend/driver/include"
export CGO_LDFLAGS="-L/usr/local/Ascend/driver/lib64"
go build
```

### Runtime Errors

**Error: Failed to initialize device manager**
- Check if Ascend driver is installed: `cat /proc/driver/ascend/version`
- Check if devices are accessible: `ls -l /dev/davinci*`
- Check user permissions: `groups | grep HwHiAiUser`

**Error: No NPU devices found**
- Verify hardware installation: `lspci | grep -i ascend`
- Check driver status: `systemctl status ascend-driver`
- Run diagnostic tool: `npu-smi info`

## License

This SDK is licensed under the Apache License 2.0.

See the original repository for full license details:
https://gitcode.com/Ascend/mind-cluster/blob/master/LICENSE

## References

- [Ascend mind-cluster Repository](https://gitcode.com/Ascend/mind-cluster)
- [Ascend Official Documentation](https://www.hiascend.com/)
- [NPU Plugin Documentation](../../inputs/npu/README.md)
- [Vendor Information](./VENDOR_INFO.md)
