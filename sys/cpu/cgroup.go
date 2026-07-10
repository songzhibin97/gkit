package cpu

import (
	"bufio"
	"fmt"
	"io"
	"math/bits"
	"os"
	"path"
	"strconv"
	"strings"
)

// cGroup LinuxCGroup
type cGroup struct {
	cGroupSet         map[string]string
	unified           bool
	unifiedMountPoint string
}

// CPUCFSQuotaUs 获取 cpu.cfs_quota_us 调度周期控制组被允许运行的时间
func (c *cGroup) CPUCFSQuotaUs() (int64, error) {
	if c.unified {
		quota, _, err := c.cpuMax()
		if err != nil {
			return 0, err
		}
		return quota, nil
	}
	data, err := readFile(path.Join(c.cGroupSet["cpu"], "cpu.cfs_quota_us"))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(data, 10, 64)
}

// CPUCFSPeriodUs 获取 cpu.cfs_period_us 调度周期
func (c *cGroup) CPUCFSPeriodUs() (uint64, error) {
	if c.unified {
		_, period, err := c.cpuMax()
		if err != nil {
			return 0, err
		}
		return period, nil
	}
	data, err := readFile(path.Join(c.cGroupSet["cpu"], "cpu.cfs_period_us"))
	if err != nil {
		return 0, err
	}
	return parseUint(data)
}

// CPUAcctUsage cpuacct.usage CPU使用率
func (c *cGroup) CPUAcctUsage() (uint64, error) {
	if c.unified {
		data, err := readFile(path.Join(c.cGroupSet[""], "cpu.stat"))
		if err != nil {
			return 0, err
		}
		for _, line := range strings.Split(data, "\n") {
			fields := strings.Fields(line)
			if len(fields) != 2 || fields[0] != "usage_usec" {
				continue
			}
			usage, err := parseUint(fields[1])
			if err != nil {
				return 0, err
			}
			if usage > ^uint64(0)/1000 {
				return 0, fmt.Errorf("cgroup v2 usage_usec overflows nanoseconds: %d", usage)
			}
			return usage * 1000, nil
		}
		return 0, fmt.Errorf("cgroup v2 cpu.stat has no usage_usec")
	}
	data, err := readFile(path.Join(c.cGroupSet["cpuacct"], "cpuacct.usage"))
	if err != nil {
		return 0, err
	}
	return parseUint(data)
}

// CPUAcctUsagePerCPU cpuacct.usage_percpu cGroup中所有任务消耗的cpu时间
func (c *cGroup) CPUAcctUsagePerCPU() ([]uint64, error) {
	if c.unified {
		return c.CPUSetCPUs()
	}
	data, err := readFile(path.Join(c.cGroupSet["cpuacct"], "cpuacct.usage_percpu"))
	if err != nil {
		return nil, err
	}
	var usage []uint64
	for _, v := range strings.Fields(string(data)) {
		var u uint64
		if u, err = parseUint(v); err != nil {
			return nil, err
		}
		if u != 0 {
			usage = append(usage, u)
		}
	}
	return usage, nil
}

// CPUSetCPUs cpuset.cpus 获取核心数
func (c *cGroup) CPUSetCPUs() ([]uint64, error) {
	base := c.cGroupSet["cpuset"]
	filename := "cpuset.cpus"
	if c.unified {
		base = c.cGroupSet[""]
		filename = "cpuset.cpus.effective"
	}
	data, err := readFile(path.Join(base, filename))
	if c.unified && (err != nil || data == "") {
		data, err = readFile(path.Join(base, "cpuset.cpus"))
	}
	if err != nil {
		return nil, err
	}
	cpus, err := ParseUintList(data)
	if err != nil {
		return nil, err
	}
	var sets []uint64
	for k := range cpus {
		sets = append(sets, uint64(k))
	}
	return sets, nil
}

// currentcGroup 获取当当前进程的 cGroup
func currentcGroup() (*cGroup, error) {
	pid := os.Getpid()
	cgroupFile := fmt.Sprintf("/proc/%d/cgroup", pid)
	cgroupFP, err := os.Open(cgroupFile)
	if err != nil {
		return nil, err
	}
	defer cgroupFP.Close()

	mountInfoFile := fmt.Sprintf("/proc/%d/mountinfo", pid)
	mountInfoFP, err := os.Open(mountInfoFile)
	if err != nil {
		return nil, err
	}
	defer mountInfoFP.Close()

	return parseCGroup(cgroupFP, mountInfoFP)
}

type cGroupMount struct {
	root       string
	mountPoint string
	fsType     string
	options    map[string]struct{}
}

func parseCGroup(cgroupReader, mountInfoReader io.Reader) (*cGroup, error) {
	controllerPaths := make(map[string]string)
	var unifiedPath string
	hasUnified := false
	scanner := bufio.NewScanner(cgroupReader)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		col := strings.SplitN(line, ":", 3)
		if len(col) != 3 {
			return nil, fmt.Errorf("invalid cGroup format %s", line)
		}
		controllers := col[1]
		membershipPath := path.Clean(col[2])
		if !strings.HasPrefix(membershipPath, "/") {
			return nil, fmt.Errorf("cgroup membership path is not absolute: %q", col[2])
		}
		if controllers == "" {
			hasUnified = true
			unifiedPath = membershipPath
			continue
		}
		for _, controller := range strings.Split(controllers, ",") {
			controllerPaths[controller] = membershipPath
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read cgroup membership: %w", err)
	}
	if len(controllerPaths) == 0 && !hasUnified {
		return nil, fmt.Errorf("no cgroup membership found")
	}

	mounts, err := parseCGroupMounts(mountInfoReader)
	if err != nil {
		return nil, err
	}
	cgroupSet := make(map[string]string)
	if cpuPath, ok := controllerPaths["cpu"]; ok {
		for controller, membershipPath := range controllerPaths {
			mappedPath, _, found := resolveCGroupPath(mounts, "cgroup", controller, membershipPath)
			if found {
				cgroupSet[controller] = mappedPath
			}
		}
		if _, ok := cgroupSet["cpu"]; !ok {
			return nil, fmt.Errorf("no visible cgroup v1 cpu mount contains membership %q", cpuPath)
		}
		return &cGroup{cGroupSet: cgroupSet}, nil
	}

	if !hasUnified {
		return nil, fmt.Errorf("no cpu cgroup membership found")
	}
	mappedPath, mountPoint, found := resolveCGroupPath(mounts, "cgroup2", "", unifiedPath)
	if !found {
		return nil, fmt.Errorf("no visible cgroup v2 mount contains membership %q", unifiedPath)
	}
	cgroupSet[""] = mappedPath
	return &cGroup{cGroupSet: cgroupSet, unified: true, unifiedMountPoint: mountPoint}, nil
}

func parseCGroupMounts(r io.Reader) ([]cGroupMount, error) {
	var mounts []cGroupMount
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
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
			return nil, fmt.Errorf("invalid mountinfo format %q", line)
		}
		fsType := fields[separator+1]
		if fsType != "cgroup" && fsType != "cgroup2" {
			continue
		}
		root, err := unescapeMountInfoPath(fields[3])
		if err != nil {
			return nil, fmt.Errorf("invalid mountinfo root %q: %w", fields[3], err)
		}
		mountPoint, err := unescapeMountInfoPath(fields[4])
		if err != nil {
			return nil, fmt.Errorf("invalid mountinfo mount point %q: %w", fields[4], err)
		}
		root = path.Clean(root)
		mountPoint = path.Clean(mountPoint)
		if !strings.HasPrefix(root, "/") || !strings.HasPrefix(mountPoint, "/") {
			return nil, fmt.Errorf("cgroup mount paths must be absolute: root=%q mountpoint=%q", root, mountPoint)
		}
		options := make(map[string]struct{})
		for _, list := range []string{fields[5], fields[separator+3]} {
			for _, option := range strings.Split(list, ",") {
				options[option] = struct{}{}
			}
		}
		mounts = append(mounts, cGroupMount{
			root:       root,
			mountPoint: mountPoint,
			fsType:     fsType,
			options:    options,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read mountinfo: %w", err)
	}
	return mounts, nil
}

func unescapeMountInfoPath(value string) (string, error) {
	var result strings.Builder
	result.Grow(len(value))
	for i := 0; i < len(value); {
		if value[i] != '\\' {
			result.WriteByte(value[i])
			i++
			continue
		}
		if i+3 >= len(value) {
			return "", fmt.Errorf("truncated escape")
		}
		var decoded byte
		switch value[i+1 : i+4] {
		case "040":
			decoded = ' '
		case "011":
			decoded = '\t'
		case "012":
			decoded = '\n'
		case "134":
			decoded = '\\'
		default:
			return "", fmt.Errorf("unsupported escape %q", value[i:i+4])
		}
		result.WriteByte(decoded)
		i += 4
	}
	return result.String(), nil
}

func resolveCGroupPath(mounts []cGroupMount, fsType, controller, membershipPath string) (string, string, bool) {
	bestRootLength := -1
	var bestPath, bestMountPoint string
	for _, mount := range mounts {
		if mount.fsType != fsType {
			continue
		}
		if controller != "" {
			if _, ok := mount.options[controller]; !ok {
				continue
			}
		}
		relative, ok := relativeCGroupPath(membershipPath, mount.root)
		if !ok || len(mount.root) <= bestRootLength {
			continue
		}
		bestRootLength = len(mount.root)
		bestPath = mount.mountPoint
		if relative != "" {
			bestPath = path.Join(bestPath, relative)
		}
		bestMountPoint = mount.mountPoint
	}
	return bestPath, bestMountPoint, bestRootLength >= 0
}

func relativeCGroupPath(member, root string) (string, bool) {
	member = path.Clean(member)
	root = path.Clean(root)
	if member == root {
		return "", true
	}
	if root == "/" {
		return strings.TrimPrefix(member, "/"), strings.HasPrefix(member, "/")
	}
	if strings.HasPrefix(member, root+"/") {
		return strings.TrimPrefix(member, root+"/"), true
	}
	return "", false
}

func (c *cGroup) cpuMax() (int64, uint64, error) {
	current := path.Clean(c.cGroupSet[""])
	mountPoint := path.Clean(c.unifiedMountPoint)
	if _, ok := relativeCGroupPath(current, mountPoint); !ok {
		return 0, 0, fmt.Errorf("cgroup v2 path %q is outside mount root %q", current, mountPoint)
	}

	bestQuota := int64(-1)
	var bestPeriod, leafPeriod uint64
	for {
		data, err := readFile(path.Join(current, "cpu.max"))
		if err != nil {
			return 0, 0, fmt.Errorf("read cgroup v2 cpu.max in %q: %w", current, err)
		}
		quota, period, err := parseCPUMax(data)
		if err != nil {
			return 0, 0, fmt.Errorf("parse cgroup v2 cpu.max in %q: %w", current, err)
		}
		if leafPeriod == 0 {
			leafPeriod = period
		}
		if quota != -1 && (bestQuota == -1 || quotaRatioLess(quota, period, bestQuota, bestPeriod)) {
			bestQuota = quota
			bestPeriod = period
		}
		if current == mountPoint {
			break
		}
		parent := path.Dir(current)
		if parent == current {
			return 0, 0, fmt.Errorf("cgroup v2 path %q did not reach mount root %q", c.cGroupSet[""], mountPoint)
		}
		current = parent
	}
	if bestQuota == -1 {
		return -1, leafPeriod, nil
	}
	return bestQuota, bestPeriod, nil
}

func parseCPUMax(data string) (int64, uint64, error) {
	fields := strings.Fields(data)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("invalid format %q", data)
	}
	period, err := parseUint(fields[1])
	if err != nil {
		return 0, 0, err
	}
	if period == 0 {
		return 0, 0, fmt.Errorf("period must be positive")
	}
	if fields[0] == "max" {
		return -1, period, nil
	}
	quota, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	if quota <= 0 {
		return 0, 0, fmt.Errorf("quota must be positive")
	}
	return quota, period, nil
}

func quotaRatioLess(quota int64, period uint64, otherQuota int64, otherPeriod uint64) bool {
	leftHigh, leftLow := bits.Mul64(uint64(quota), otherPeriod)
	rightHigh, rightLow := bits.Mul64(uint64(otherQuota), period)
	return leftHigh < rightHigh || leftHigh == rightHigh && leftLow < rightLow
}
