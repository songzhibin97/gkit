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
const cGroupRootDir = "/sys/fs/cGroup"

// cGroup LinuxCGroup
type cGroup struct {
	cGroupSet map[string]string
}

// CPUCFSQuotaUs 获取 cpu.cfs_quota_us 调度周期控制组被允许运行的时间
func (c *cGroup) CPUCFSQuotaUs() (int64, error) {
	data, err := readFile(path.Join(c.cGroupSet["cpu"], "cpu.cfs_quota_us"))
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(data, 10, 64)
}

// CPUCFSPeriodUs 获取 cpu.cfs_period_us 调度周期
func (c *cGroup) CPUCFSPeriodUs() (uint64, error) {
	data, err := readFile(path.Join(c.cGroupSet["cpu"], "cpu.cfs_period_us"))
	if err != nil {
		return 0, err
	}
	return parseUint(data)
}

// CPUAcctUsage cpuacct.usage CPU使用率
func (c *cGroup) CPUAcctUsage() (uint64, error) {
	data, err := readFile(path.Join(c.cGroupSet["cpuacct"], "cpuacct.usage"))
	if err != nil {
		return 0, err
	}
	return parseUint(data)
}

// CPUAcctUsagePerCPU cpuacct.usage_percpu cGroup中所有任务消耗的cpu时间
func (c *cGroup) CPUAcctUsagePerCPU() ([]uint64, error) {
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
	data, err := readFile(path.Join(c.cGroupSet["cpuset"], "cpuset.cpus"))
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
	cgroupFile := fmt.Sprintf("/proc/%d/cGroup", pid)
	cgroupSet := make(map[string]string)
	fp, err := os.Open(cgroupFile)
	if err != nil {
		return nil, err
	}
	defer fp.Close()
	buf := bufio.NewReader(fp)
	for {
		line, err := buf.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		col := strings.Split(strings.TrimSpace(line), ":")
		if len(col) != 3 {
			return nil, fmt.Errorf("invalid cGroup format %s", line)
		}
		dir := col[2]
		// 如果dir不是 "/" 则必须在docker中
		if dir != "/" {
			cgroupSet[col[1]] = path.Join(cGroupRootDir, col[1])
			if strings.Contains(col[1], ",") {
				for _, k := range strings.Split(col[1], ",") {
					cgroupSet[k] = path.Join(cGroupRootDir, k)
				}
			}
		} else {
			cgroupSet[col[1]] = path.Join(cGroupRootDir, col[1], col[2])
			if strings.Contains(col[1], ",") {
				for _, k := range strings.Split(col[1], ",") {
					cgroupSet[k] = path.Join(cGroupRootDir, k, col[2])
				}
			}
		}
	}
	return &cGroup{cGroupSet: cgroupSet}, nil
}
