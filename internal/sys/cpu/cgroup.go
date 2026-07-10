package cpu

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

// cGroupRootDir 目录位置
const cGroupRootDir = "/sys/fs/cgroup"

// cGroup LinuxCGroup
type cGroup struct {
	cGroupSet map[string]string
	unified   bool
}

// CPUCFSQuotaUs 获取 cpu.cfs_quota_us 调度周期控制组被允许运行的时间
func (c *cGroup) CPUCFSQuotaUs() (int64, error) {
	if c.unified {
		fields, err := c.cpuMax()
		if err != nil {
			return 0, err
		}
		if fields[0] == "max" {
			return -1, nil
		}
		return strconv.ParseInt(fields[0], 10, 64)
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
		fields, err := c.cpuMax()
		if err != nil {
			return 0, err
		}
		return parseUint(fields[1])
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
	fp, err := os.Open(cgroupFile)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	return parseCGroup(fp, cGroupRootDir)
}

func parseCGroup(r io.Reader, root string) (*cGroup, error) {
	cgroupSet := make(map[string]string)
	unified := false
	scanner := bufio.NewScanner(r)
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
		if controllers == "" {
			unified = true
			dir := strings.TrimPrefix(col[2], "/")
			cgroupSet[""] = path.Join(root, dir)
			continue
		}
		// Preserve the package's existing cgroup v1 mount mapping. Controller
		// names and membership paths do not identify the actual v1 mountpoint;
		// changing this requires parsing mountinfo rather than guessing from the
		// /proc membership line.
		cgroupSet[controllers] = path.Join(root, controllers)
		for _, controller := range strings.Split(controllers, ",") {
			cgroupSet[controller] = path.Join(root, controller)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read cgroup membership: %w", err)
	}
	if len(cgroupSet) == 0 {
		return nil, fmt.Errorf("no cgroup membership found")
	}
	return &cGroup{cGroupSet: cgroupSet, unified: unified}, nil
}

func (c *cGroup) cpuMax() ([]string, error) {
	data, err := readFile(path.Join(c.cGroupSet[""], "cpu.max"))
	if err != nil {
		return nil, err
	}
	fields := strings.Fields(data)
	if len(fields) != 2 {
		return nil, fmt.Errorf("invalid cgroup v2 cpu.max format %q", data)
	}
	return fields, nil
}
