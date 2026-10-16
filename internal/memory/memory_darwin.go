// Package memory reads available RAM and swap activity from the kernel, without parsing vm_stat.
package memory

/*
#include <mach/mach.h>

static kern_return_t vmstat64(vm_statistics64_data_t *s, vm_size_t *page) {
	mach_msg_type_number_t n = HOST_VM_INFO64_COUNT;
	host_page_size(mach_host_self(), page);
	return host_statistics64(mach_host_self(), HOST_VM_INFO64, (host_info64_t)s, &n);
}
*/
import "C"

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

type Stats struct {
	// AvailableBytes counts free, inactive, speculative and purgeable pages: what a new process can get
	// without pushing anything to swap.
	AvailableBytes uint64
	TotalBytes     uint64
	// Swapouts is the cumulative number of pages written to swap since boot. Pageouts are file-backed
	// pages and must not be counted as swap.
	Swapouts  uint64
	SwapUsed  uint64
	SwapTotal uint64
	PageSize  uint64
}

func Read() (Stats, error) {
	var s C.vm_statistics64_data_t
	var page C.vm_size_t
	if kr := C.vmstat64(&s, &page); kr != C.KERN_SUCCESS {
		return Stats{}, fmt.Errorf("host_statistics64: kern_return %d", kr)
	}
	ps := uint64(page)
	st := Stats{
		AvailableBytes: (uint64(s.free_count) + uint64(s.inactive_count) + uint64(s.speculative_count) + uint64(s.purgeable_count)) * ps,
		Swapouts:       uint64(s.swapouts),
		PageSize:       ps,
	}
	if total, err := unix.SysctlUint64("hw.memsize"); err == nil {
		st.TotalBytes = total
	}
	// struct xsw_usage { u_int64_t xsu_total, xsu_avail, xsu_used; u_int32_t xsu_pagesize; boolean_t xsu_encrypted; }
	if raw, err := unix.SysctlRaw("vm.swapusage"); err == nil && len(raw) >= 24 {
		st.SwapTotal = binary.LittleEndian.Uint64(raw[0:8])
		st.SwapUsed = binary.LittleEndian.Uint64(raw[16:24])
	}
	return st, nil
}

func GB(b uint64) float64 { return float64(b) / (1 << 30) }
func MB(b uint64) float64 { return float64(b) / (1 << 20) }
