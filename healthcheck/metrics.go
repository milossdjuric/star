package healthcheck

import (
	"errors"
	mPb "github.com/c12s/scheme/magnetar"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"strconv"
	"strings"
	"time"
)

func now() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func (ht *Healthcheck) metrics() (*mPb.EventMsg, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	a, err := load.Avg()
	if err != nil {
		return nil, err
	}

	c, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	cs, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}

	if len(cs) == 0 {
		return nil, errors.New("No cpu stat data")
	}
	ct := cs[0]

	d, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	h, err := host.Info()
	if err != nil {
		return nil, err
	}

	data := map[string]*mPb.DataMsg{
		"general": &mPb.DataMsg{
			Data: map[string]string{
				"key":    topology(ht.nodeId),
				"policy": "2h",
			},
		},
		"tags": &mPb.DataMsg{
			Data: map[string]string{
				"mem":  "mem-total",
				"load": "load-total",
				"disk": "disk-total",
				"cpu":  "cpu-total",
			},
		},
		"points": &mPb.DataMsg{
			Data: map[string]string{
				"mem":  "mem_usage",
				"load": "load_usage",
				"disk": "disk_usage",
				"cpu":  "cpu_usage",
			},
		},
		"times": &mPb.DataMsg{
			Data: map[string]string{
				"time": now(),
			},
		},
		"mem": &mPb.DataMsg{
			Data: map[string]string{
				"total": strconv.FormatUint(v.Total, 10),
				"used":  strconv.FormatUint(v.Used, 10),
				"free":  strconv.FormatUint(v.Available, 10),
			},
		},
		"load": &mPb.DataMsg{
			Data: map[string]string{
				"1s":  strconv.FormatFloat(a.Load1, 'E', -1, 64),
				"5s":  strconv.FormatFloat(a.Load5, 'E', -1, 64),
				"15s": strconv.FormatFloat(a.Load15, 'E', -1, 64),
			},
		},
		"disk": &mPb.DataMsg{
			Data: map[string]string{
				"total": strconv.FormatUint(d.Total, 10),
				"used":  strconv.FormatUint(d.Used, 10),
				"free":  strconv.FormatUint(d.Free, 10),
			},
		},
		"cpu": &mPb.DataMsg{
			Data: map[string]string{
				"user":      strconv.FormatFloat(ct.User, 'E', -1, 64),
				"system":    strconv.FormatFloat(ct.System, 'E', -1, 64),
				"idle":      strconv.FormatFloat(ct.Idle, 'E', -1, 64),
				"nice":      strconv.FormatFloat(ct.Nice, 'E', -1, 64),
				"iowait":    strconv.FormatFloat(ct.Iowait, 'E', -1, 64),
				"irq":       strconv.FormatFloat(ct.Irq, 'E', -1, 64),
				"softirq":   strconv.FormatFloat(ct.Softirq, 'E', -1, 64),
				"steal":     strconv.FormatFloat(ct.Steal, 'E', -1, 64),
				"guest":     strconv.FormatFloat(ct.Guest, 'E', -1, 64),
				"guestNice": strconv.FormatFloat(ct.GuestNice, 'E', -1, 64),
			},
		},
	}

	// if ht.hostParams {
	data["host"] = &mPb.DataMsg{
		Data: map[string]string{
			"os":                 h.OS,
			"platform":           h.Platform,
			"platformFamily":     h.PlatformFamily,
			"platformVersion":    h.PlatformVersion,
			"kernelVersion":      h.KernelVersion,
			"kernelArchitecture": h.KernelArch,
			"virtualization":     h.VirtualizationSystem,
			"virtualizationRole": h.VirtualizationRole,
		},
	}

	if !used(ht.nodeId) {
		data["labels"] = &mPb.DataMsg{
			Data: ht.labels,
		}
	}

	for i, t := range c {
		num := strconv.FormatInt(int64(i)+1, 10)
		data["host"].Data[strings.Join([]string{"cpu", num, "model"}, ".")] = t.ModelName
		data["host"].Data[strings.Join([]string{"cpu", num, "cores"}, ".")] = strconv.FormatInt(int64(t.Cores), 10)
		data["host"].Data[strings.Join([]string{"cpu", num, "mhz"}, ".")] = strconv.FormatFloat(t.Mhz, 'E', -1, 64)
	}
	// }
	return &mPb.EventMsg{Data: data}, nil
}
