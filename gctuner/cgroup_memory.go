package gctuner

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func getCGroupMemoryLimit() (uint64, error) {
	membership, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return 0, fmt.Errorf("read cgroup membership: %w", err)
	}
	mounts, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		return 0, fmt.Errorf("read cgroup mounts: %w", err)
	}
	host, err := getNormalMemoryLimit()
	if err != nil {
		return 0, fmt.Errorf("read host memory: %w", err)
	}
	return cgroupMemoryLimit(string(membership), string(mounts), host)
}

// cgroupMemoryLimit follows the process's memory controller to its visible
// mount, then bounds its limit by visible ancestors and physical host memory.
func cgroupMemoryLimit(membership, mountinfo string, host uint64) (uint64, error) {
	var v1, v2 string
	for _, line := range strings.Split(membership, "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, ":", 3)
		if len(fields) != 3 || !filepath.IsAbs(fields[2]) {
			return 0, fmt.Errorf("invalid cgroup membership %q", line)
		}
		if fields[1] == "" {
			v2 = filepath.Clean(fields[2])
		}
		if memoryController(fields[1]) {
			v1 = filepath.Clean(fields[2])
		}
	}
	member, fsType, filename := v1, "cgroup", "memory.limit_in_bytes"
	if member == "" {
		member, fsType, filename = v2, "cgroup2", "memory.max"
	}
	if member == "" {
		return 0, fmt.Errorf("no memory cgroup membership")
	}

	var mountPoint, mountRoot, current string
	for _, line := range strings.Split(mountinfo, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		separator := -1
		for i, field := range fields {
			if field == "-" {
				separator = i
				break
			}
		}
		if separator < 6 || len(fields) < separator+4 {
			return 0, fmt.Errorf("invalid mountinfo record %q", line)
		}
		if fields[separator+1] != fsType {
			continue
		}
		if fsType == "cgroup" && !memoryController(fields[separator+3]) {
			continue
		}
		root, err := memoryMountPath(fields[3])
		if err != nil {
			return 0, err
		}
		mount, err := memoryMountPath(fields[4])
		if err != nil {
			return 0, err
		}
		relative := ""
		switch {
		case member == root:
		case root == "/":
			relative = strings.TrimPrefix(member, "/")
		case strings.HasPrefix(member, root+"/"):
			relative = strings.TrimPrefix(member, root+"/")
		default:
			continue
		}
		// Prefer the broadest visible hierarchy so a bind mount cannot hide an
		// ancestor's tighter limit when another mount exposes that ancestor.
		if mountPoint == "" || len(root) < len(mountRoot) {
			mountPoint, mountRoot, current = mount, root, filepath.Join(mount, relative)
		}
	}
	if mountPoint == "" {
		return 0, fmt.Errorf("no visible %s memory mount contains membership %q", fsType, member)
	}
	limit := host
	leaf := current
	for {
		if fsType == "cgroup" && current != leaf {
			// v1 ancestors only account descendants with hierarchical accounting enabled.
			hierarchy, err := os.ReadFile(filepath.Join(current, "memory.use_hierarchy"))
			if err != nil {
				return 0, fmt.Errorf("read cgroup memory hierarchy in %q: %w", current, err)
			}
			switch strings.TrimSpace(string(hierarchy)) {
			case "0":
				if current == mountPoint {
					return limit, nil
				}
				current = filepath.Dir(current)
				continue
			case "1":
			default:
				return 0, fmt.Errorf("invalid cgroup memory hierarchy in %q", current)
			}
		}
		name := filepath.Join(current, filename)
		data, err := os.ReadFile(name)
		// memory.max can be absent at a v2 hierarchy root. Accept that only
		// at a visible mount root reported as "/", with memory listed as
		// an available controller. Hidden ancestor limits cannot be inspected.
		if err != nil && fsType == "cgroup2" && current == mountPoint && mountRoot == "/" && os.IsNotExist(err) {
			controllers, controllerErr := os.ReadFile(filepath.Join(current, "cgroup.controllers"))
			if controllerErr != nil {
				return 0, fmt.Errorf("read cgroup memory root (%v): %w", err, controllerErr)
			}
			hasMemory := false
			for _, controller := range strings.Fields(string(controllers)) {
				if controller == "memory" {
					hasMemory = true
					break
				}
			}
			if !hasMemory {
				return 0, fmt.Errorf("no memory controller at visible cgroup root %q", current)
			}
		} else {
			if err != nil {
				return 0, fmt.Errorf("read cgroup memory limit %q: %w", name, err)
			}
			value := strings.TrimSpace(string(data))
			if !(fsType == "cgroup2" && value == "max") && !(fsType == "cgroup" && value == "-1") {
				cap, err := strconv.ParseUint(value, 10, 64)
				if err != nil {
					return 0, fmt.Errorf("parse cgroup memory limit %q: %w", name, err)
				}
				if cap < limit {
					limit = cap
				}
			}
		}
		if current == mountPoint {
			return limit, nil
		}
		current = filepath.Dir(current)
	}
}

func memoryController(controllers string) bool {
	for _, controller := range strings.Split(controllers, ",") {
		if controller == "memory" {
			return true
		}
	}
	return false
}

func memoryMountPath(value string) (string, error) {
	var decoded strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			decoded.WriteByte(value[i])
			continue
		}
		if i+3 >= len(value) {
			return "", fmt.Errorf("truncated mount path escape %q", value)
		}
		switch value[i+1 : i+4] {
		case "040":
			decoded.WriteByte(' ')
		case "011":
			decoded.WriteByte('\t')
		case "012":
			decoded.WriteByte('\n')
		case "134":
			decoded.WriteByte('\\')
		default:
			return "", fmt.Errorf("invalid mount path escape %q", value)
		}
		i += 3
	}
	result := filepath.Clean(decoded.String())
	if !filepath.IsAbs(result) {
		return "", fmt.Errorf("mount path is not absolute: %q", value)
	}
	return result, nil
}
