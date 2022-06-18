package main

import (
	"math"

	"github.com/shirou/gopsutil/disk"
)

func DiskSize(opt []string) []*disk.UsageStat {
	ps, _ := disk.Partitions(true)
	var list []string
	for _, p := range ps {
		if len(opt) > 0 {
			for _, o := range opt {
				if o == p.Mountpoint {
					list = append(list, p.Mountpoint)
				}
			}
		} else {
			list = append(list, p.Mountpoint)
		}
	}
	var output []*disk.UsageStat
	for _, v := range list {
		u, err := disk.Usage(v)
		if err != nil {
			continue
		}
		u.UsedPercent = math.Round(u.UsedPercent*100) / 100
		u.InodesUsedPercent = math.Round(u.InodesUsedPercent*100) / 100
		output = append(output, u)
	}
	return output
}
